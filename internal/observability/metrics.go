package observability

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/alexagc/kalyx/internal/storage"
)

// DefaultHistogramBuckets provides standard latency buckets in seconds.
var DefaultHistogramBuckets = []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5}

// MetricRequestKey uniquely identifies an HTTP counter metric series.
type MetricRequestKey struct {
	Method string
	Path   string
	Code   int
}

// MetricContractKey uniquely identifies a contracts_total gauge series.
type MetricContractKey struct {
	Domain string
	Status string
}

// Registry manages in-memory Prometheus metrics with thread safety and zero external dependencies.
type Registry struct {
	mu sync.RWMutex

	// http_requests_total
	httpRequests map[MetricRequestKey]int64

	// validation_duration_seconds
	validationBuckets []float64
	validationCounts  map[float64]int64
	validationSum     float64
	validationTotal   int64

	// contracts_total
	contractsTotal map[MetricContractKey]int64
}

// DefaultRegistry is the singleton metrics registry for the application.
var DefaultRegistry = NewRegistry()

// NewRegistry initializes a clean, empty metrics registry.
func NewRegistry() *Registry {
	buckets := make([]float64, len(DefaultHistogramBuckets))
	copy(buckets, DefaultHistogramBuckets)
	sort.Float64s(buckets)

	r := &Registry{
		httpRequests:      make(map[MetricRequestKey]int64),
		validationBuckets: buckets,
		validationCounts:  make(map[float64]int64),
		contractsTotal:    make(map[MetricContractKey]int64),
	}
	for _, b := range buckets {
		r.validationCounts[b] = 0
	}
	return r
}

// IncHTTPRequests increments the http_requests_total counter.
func (r *Registry) IncHTTPRequests(method, path string, code int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := MetricRequestKey{
		Method: method,
		Path:   normalizeMetricPath(path),
		Code:   code,
	}
	r.httpRequests[key]++
}

// ObserveValidationDuration records a schema validation latency observation in seconds.
func (r *Registry) ObserveValidationDuration(durationSeconds float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.validationTotal++
	r.validationSum += durationSeconds

	for _, b := range r.validationBuckets {
		if durationSeconds <= b {
			r.validationCounts[b]++
		}
	}
}

// SetContractsTotal sets the value of the contracts_total gauge for a domain and status.
func (r *Registry) SetContractsTotal(domain, status string, count int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := MetricContractKey{
		Domain: domain,
		Status: status,
	}
	r.contractsTotal[key] = count
}

// SyncContractsFromStorage updates the contracts_total gauge based on live storage state.
func (r *Registry) SyncContractsFromStorage(ctx context.Context, store storage.Storage) error {
	if store == nil {
		return nil
	}

	events, err := store.ListEvents(ctx)
	if err != nil {
		return err
	}

	counts := make(map[MetricContractKey]int64)

	for _, ev := range events {
		domain := ev.Domain
		if domain == "" {
			domain = "default"
		}

		versions, err := store.ListVersions(ctx, ev.ID)
		if err != nil || len(versions) == 0 {
			key := MetricContractKey{Domain: domain, Status: "registered"}
			counts[key]++
			continue
		}

		for _, v := range versions {
			status := string(v.Status)
			if status == "" {
				status = "unknown"
			}
			key := MetricContractKey{Domain: domain, Status: status}
			counts[key]++
		}
	}

	r.mu.Lock()
	r.contractsTotal = counts
	r.mu.Unlock()

	return nil
}

// Handler returns an http.HandlerFunc that exports metrics in Prometheus text format (0.0.4).
func (r *Registry) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		r.mu.RLock()
		defer r.mu.RUnlock()

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		var b strings.Builder

		// 1. http_requests_total
		b.WriteString("# HELP http_requests_total Total number of HTTP requests processed by Kalyx.\n")
		b.WriteString("# TYPE http_requests_total counter\n")
		for key, val := range r.httpRequests {
			b.WriteString(fmt.Sprintf("http_requests_total{code=\"%d\",method=\"%s\",path=\"%s\"} %d\n", key.Code, key.Method, key.Path, val))
		}
		b.WriteString("\n")

		// 2. validation_duration_seconds
		b.WriteString("# HELP validation_duration_seconds Latency of CloudEvent JSON Schema validations in seconds.\n")
		b.WriteString("# TYPE validation_duration_seconds histogram\n")
		for _, bucket := range r.validationBuckets {
			count := r.validationCounts[bucket]
			b.WriteString(fmt.Sprintf("validation_duration_seconds_bucket{le=\"%s\"} %d\n", strconv.FormatFloat(bucket, 'f', -1, 64), count))
		}
		b.WriteString(fmt.Sprintf("validation_duration_seconds_bucket{le=\"+Inf\"} %d\n", r.validationTotal))
		b.WriteString(fmt.Sprintf("validation_duration_seconds_sum %s\n", strconv.FormatFloat(r.validationSum, 'f', 6, 64)))
		b.WriteString(fmt.Sprintf("validation_duration_seconds_count %d\n", r.validationTotal))
		b.WriteString("\n")

		// 3. contracts_total
		b.WriteString("# HELP contracts_total Total number of registered event contracts in Kalyx.\n")
		b.WriteString("# TYPE contracts_total gauge\n")
		for key, count := range r.contractsTotal {
			b.WriteString(fmt.Sprintf("contracts_total{domain=\"%s\",status=\"%s\"} %d\n", key.Domain, key.Status, count))
		}

		_, _ = w.Write([]byte(b.String()))
	}
}

// HTTPMiddleware automatically instruments requests with latency and status code tracking.
func (r *Registry) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		rw := &responseWriterInterceptor{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, req)
		r.IncHTTPRequests(req.Method, req.URL.Path, rw.statusCode)
	})
}

type responseWriterInterceptor struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriterInterceptor) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// normalizeMetricPath aggregates dynamic path parameters to avoid high Prometheus cardinality.
func normalizeMetricPath(p string) string {
	if strings.HasPrefix(p, "/api/v1/events/") {
		parts := strings.Split(strings.Trim(p, "/"), "/")
		// parts: ["api", "v1", "events", "{id}", ...]
		if len(parts) >= 7 && parts[6] == "ddl" {
			return "/api/v1/events/:id/versions/:semver/ddl"
		}
		if len(parts) >= 6 && parts[4] == "versions" {
			return "/api/v1/events/:id/versions/:semver"
		}
		if len(parts) == 5 && parts[4] == "versions" {
			return "/api/v1/events/:id/versions"
		}
		if len(parts) == 4 {
			return "/api/v1/events/:id"
		}
	}
	return p
}
