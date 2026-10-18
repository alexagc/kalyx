package webhooks

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/alexagc/kalyx/internal/compatibility"
	"github.com/alexagc/kalyx/internal/core"
	"github.com/alexagc/kalyx/internal/datalake"
)

// EventTypeContractApproved is the standard CloudEvents type for contract approval notifications.
const EventTypeContractApproved = "org.kalyx.contract.approved.v1"

// RetryConfig defines the parameters for exponential backoff retries.
type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	BackoffFactor  float64
}

// DefaultRetryConfig returns sensible defaults for production dispatch.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     2 * time.Second,
		BackoffFactor:  2.0,
	}
}

// ContractApprovedData defines the business data payload of the CloudEvent.
type ContractApprovedData struct {
	EventID            string                             `json:"event_id"`
	Domain             string                             `json:"domain"`
	Name               string                             `json:"name"`
	CloudEventType     string                             `json:"cloudevent_type"`
	SemVer             string                             `json:"semver"`
	Status             core.EventStatus                   `json:"status"`
	JSONSchema         string                             `json:"json_schema"`
	DDLs               map[string]string                  `json:"ddls"`
	SemVerChangeReport *compatibility.CompatibilityResult `json:"semver_change_report,omitempty"`
	ApprovedBy         string                             `json:"approved_by,omitempty"`
	ApprovedAt         *time.Time                         `json:"approved_at,omitempty"`
}

// CloudEventMessage represents a CloudEvents v1.0.2 event envelope.
type CloudEventMessage struct {
	SpecVersion     string               `json:"specversion"`
	ID              string               `json:"id"`
	Source          string               `json:"source"`
	Type            string               `json:"type"`
	Time            string               `json:"time"`
	DataContentType string               `json:"datacontenttype"`
	Data            ContractApprovedData `json:"data"`
}

// Dispatcher coordinates the delivery of webhook notifications with exponential backoff.
type Dispatcher struct {
	client      *http.Client
	retryConfig RetryConfig
	sourceURI   string
}

// NewDispatcher creates a new Dispatcher with optional custom retry config.
func NewDispatcher(client *http.Client, retryConfig *RetryConfig, sourceURI string) *Dispatcher {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	cfg := DefaultRetryConfig()
	if retryConfig != nil {
		cfg = *retryConfig
	}
	if sourceURI == "" {
		sourceURI = "https://kalyx.io/contracts"
	}

	return &Dispatcher{
		client:      client,
		retryConfig: cfg,
		sourceURI:   sourceURI,
	}
}

// BuildApprovedEvent builds the CloudEvents v1.0.2 message for an approved contract.
func (d *Dispatcher) BuildApprovedEvent(
	event core.EventDefinition,
	version core.EventVersion,
	changeReport *compatibility.CompatibilityResult,
) (*CloudEventMessage, error) {
	tableName := version.DataLakeMetadata.TargetTable
	if tableName == "" {
		tableName = event.Name
	}

	// Generate both Iceberg and Delta Lake DDLs
	icebergDDL, err := datalake.GenerateIcebergDDL(tableName, version.JSONSchema, version.DataLakeMetadata.PartitionBy)
	if err != nil {
		return nil, fmt.Errorf("failed generating iceberg DDL: %w", err)
	}

	deltaDDL, err := datalake.GenerateDeltaDDL(tableName, version.JSONSchema, version.DataLakeMetadata.PartitionBy)
	if err != nil {
		return nil, fmt.Errorf("failed generating delta DDL: %w", err)
	}

	ddls := map[string]string{
		datalake.FormatIceberg: icebergDDL,
		datalake.FormatDelta:   deltaDDL,
	}

	data := ContractApprovedData{
		EventID:            event.ID,
		Domain:             event.Domain,
		Name:               event.Name,
		CloudEventType:     event.Type,
		SemVer:             version.SemVer,
		Status:             version.Status,
		JSONSchema:         version.JSONSchema,
		DDLs:               ddls,
		SemVerChangeReport: changeReport,
		ApprovedBy:         version.ApprovedBy,
		ApprovedAt:         version.ApprovedAt,
	}

	eventID := generateEventID()
	now := time.Now().UTC().Format(time.RFC3339)

	msg := &CloudEventMessage{
		SpecVersion:     "1.0",
		ID:              eventID,
		Source:          d.sourceURI,
		Type:            EventTypeContractApproved,
		Time:            now,
		DataContentType: "application/json",
		Data:            data,
	}

	return msg, nil
}

// DispatchApprovedEvent creates and sends the CloudEvent to the target URL with exponential backoff retries.
func (d *Dispatcher) DispatchApprovedEvent(
	ctx context.Context,
	webhookURL string,
	event core.EventDefinition,
	version core.EventVersion,
	changeReport *compatibility.CompatibilityResult,
) error {
	msg, err := d.BuildApprovedEvent(event, version, changeReport)
	if err != nil {
		return fmt.Errorf("failed to build webhook event: %w", err)
	}

	return d.Dispatch(ctx, webhookURL, msg)
}

// Dispatch sends a CloudEventMessage to the target webhook URL with exponential backoff retries.
func (d *Dispatcher) Dispatch(ctx context.Context, webhookURL string, msg *CloudEventMessage) error {
	bodyBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal cloudevent: %w", err)
	}

	var lastErr error
	backoff := d.retryConfig.InitialBackoff

	for attempt := 0; attempt <= d.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			slog.Warn("retrying webhook dispatch",
				"url", webhookURL,
				"attempt", attempt,
				"backoff", backoff,
				"error", lastErr,
			)

			select {
			case <-ctx.Done():
				return fmt.Errorf("webhook dispatch canceled during retry backoff: %w", ctx.Err())
			case <-time.After(backoff):
			}

			// Exponential backoff with ceiling
			nextBackoff := float64(backoff) * d.retryConfig.BackoffFactor
			if nextBackoff > float64(d.retryConfig.MaxBackoff) {
				backoff = d.retryConfig.MaxBackoff
			} else {
				backoff = time.Duration(nextBackoff)
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return fmt.Errorf("failed to create http request: %w", err)
		}

		req.Header.Set("Content-Type", "application/cloudevents+json; charset=utf-8")
		req.Header.Set("User-Agent", "kalyx-webhook-dispatcher/1.0")

		resp, err := d.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("network error on attempt %d: %w", attempt+1, err)
			continue
		}

		// Read and close body to allow connection reuse
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			slog.Info("webhook successfully dispatched",
				"url", webhookURL,
				"status", resp.StatusCode,
				"event_id", msg.ID,
				"attempts", attempt+1,
			)
			return nil
		}

		lastErr = fmt.Errorf("server returned status %d on attempt %d", resp.StatusCode, attempt+1)

		// Non-retryable client errors (4xx) except 429
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return fmt.Errorf("non-retryable client error from webhook receiver: %w", lastErr)
		}
	}

	return fmt.Errorf("webhook dispatch failed after %d attempts: %w", d.retryConfig.MaxRetries+1, lastErr)
}

func generateEventID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("kalyx-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
