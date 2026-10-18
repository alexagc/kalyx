package webhooks

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alexagc/kalyx/internal/compatibility"
	"github.com/alexagc/kalyx/internal/core"
	"github.com/alexagc/kalyx/internal/datalake"
)

func TestDispatcher_Success(t *testing.T) {
	var receivedEvent CloudEventMessage
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)

		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/cloudevents+json; charset=utf-8" {
			t.Errorf("unexpected content-type: %s", r.Header.Get("Content-Type"))
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed reading body: %v", err)
		}

		if err := json.Unmarshal(body, &receivedEvent); err != nil {
			t.Errorf("failed unmarshaling cloudevent: %v", err)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"received":true}`))
	}))
	defer server.Close()

	dispatcher := NewDispatcher(server.Client(), nil, "https://kalyx.test/registry")

	event := core.EventDefinition{
		ID:        "orders.order.completed",
		Domain:    "orders",
		Name:      "order_completed",
		Type:      "com.company.orders.completed.v1",
		OwnerTeam: "checkout",
	}

	schemaJSON := `{
		"title": "OrderCompleted",
		"properties": {
			"order_id": {"type": "string"},
			"total": {"type": "number"}
		},
		"required": ["order_id"]
	}`

	now := time.Now().UTC()
	version := core.EventVersion{
		ID:         "v-1",
		EventID:    event.ID,
		SemVer:     "1.0.0",
		Status:     core.StatusApproved,
		JSONSchema: schemaJSON,
		ApprovedBy: "steward@company.com",
		ApprovedAt: &now,
	}

	changeReport := &compatibility.CompatibilityResult{
		IsCompatible: true,
		ChangeType:   compatibility.ChangePatch,
		Reasons:      []string{"Initial approval"},
	}

	err := dispatcher.DispatchApprovedEvent(context.Background(), server.URL, event, version, changeReport)
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}

	if atomic.LoadInt32(&requestCount) != 1 {
		t.Errorf("expected 1 request, got %d", requestCount)
	}

	if receivedEvent.SpecVersion != "1.0" {
		t.Errorf("expected specversion 1.0, got %s", receivedEvent.SpecVersion)
	}
	if receivedEvent.Type != EventTypeContractApproved {
		t.Errorf("expected type %s, got %s", EventTypeContractApproved, receivedEvent.Type)
	}
	if receivedEvent.Data.EventID != event.ID {
		t.Errorf("expected event_id %s, got %s", event.ID, receivedEvent.Data.EventID)
	}
	if receivedEvent.Data.DDLs[datalake.FormatIceberg] == "" {
		t.Errorf("expected Iceberg DDL in event payload")
	}
	if receivedEvent.Data.DDLs[datalake.FormatDelta] == "" {
		t.Errorf("expected Delta Lake DDL in event payload")
	}
	if receivedEvent.Data.SemVerChangeReport == nil {
		t.Errorf("expected semver change report in event payload")
	}
}

func TestDispatcher_RetrySuccess(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		att := atomic.AddInt32(&attempts, 1)
		if att < 3 {
			// Fail first two attempts with 503 Service Unavailable
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("temporary error"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	retryCfg := &RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 5 * time.Millisecond,
		MaxBackoff:     20 * time.Millisecond,
		BackoffFactor:  2.0,
	}

	dispatcher := NewDispatcher(server.Client(), retryCfg, "")

	msg := &CloudEventMessage{
		SpecVersion: "1.0",
		ID:          "test-1",
		Source:      "https://kalyx.test",
		Type:        EventTypeContractApproved,
		Time:        time.Now().UTC().Format(time.RFC3339),
		Data:        ContractApprovedData{EventID: "evt-1", SemVer: "1.0.0"},
	}

	err := dispatcher.Dispatch(context.Background(), server.URL, msg)
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}

	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts before success, got %d", attempts)
	}
}

func TestDispatcher_RetryExhausted(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	retryCfg := &RetryConfig{
		MaxRetries:     2,
		InitialBackoff: 5 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		BackoffFactor:  1.5,
	}

	dispatcher := NewDispatcher(server.Client(), retryCfg, "")

	msg := &CloudEventMessage{
		SpecVersion: "1.0",
		ID:          "test-fail",
		Source:      "https://kalyx.test",
		Type:        EventTypeContractApproved,
	}

	err := dispatcher.Dispatch(context.Background(), server.URL, msg)
	if err == nil {
		t.Fatalf("expected error after retries exhausted, got nil")
	}

	// 1 initial attempt + 2 retries = 3 attempts total
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 total attempts, got %d", attempts)
	}
}

func TestDispatcher_NonRetryableClientError(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest) // 400 Bad Request
	}))
	defer server.Close()

	retryCfg := &RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 5 * time.Millisecond,
	}

	dispatcher := NewDispatcher(server.Client(), retryCfg, "")

	msg := &CloudEventMessage{
		SpecVersion: "1.0",
		ID:          "test-400",
		Source:      "https://kalyx.test",
		Type:        EventTypeContractApproved,
	}

	err := dispatcher.Dispatch(context.Background(), server.URL, msg)
	if err == nil {
		t.Fatalf("expected error for 400 response, got nil")
	}

	// Should not retry 400 errors
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("expected only 1 attempt on non-retryable 400 error, got %d", attempts)
	}
}
