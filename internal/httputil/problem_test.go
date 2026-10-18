package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteProblem(t *testing.T) {
	rec := httptest.NewRecorder()
	invalidParams := []InvalidParam{
		{Name: "data.amount", Reason: "must be greater than 0"},
		{Name: "data.order_id", Reason: "field is required"},
	}

	WriteProblem(rec, http.StatusUnprocessableEntity, "Validation Failed", "CloudEvent payload failed schema validation", invalidParams)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected status %d, got %d", http.StatusUnprocessableEntity, rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/problem+json" {
		t.Errorf("expected Content-Type application/problem+json, got %s", contentType)
	}

	var prob ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if prob.Type != "urn:kalyx:error:validation-failed" {
		t.Errorf("expected urn:kalyx:error:validation-failed, got %s", prob.Type)
	}
	if prob.Title != "Validation Failed" {
		t.Errorf("expected title 'Validation Failed', got '%s'", prob.Title)
	}
	if prob.Status != http.StatusUnprocessableEntity {
		t.Errorf("expected status %d, got %d", http.StatusUnprocessableEntity, prob.Status)
	}
	if len(prob.InvalidParams) != 2 {
		t.Fatalf("expected 2 invalid params, got %d", len(prob.InvalidParams))
	}
	if prob.InvalidParams[0].Name != "data.amount" {
		t.Errorf("expected invalid param name 'data.amount', got '%s'", prob.InvalidParams[0].Name)
	}
}

func TestWriteProblemSimple(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteProblemSimple(rec, http.StatusNotFound, "Resource Not Found", "Contract not found")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}

	var prob ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if prob.Type != "urn:kalyx:error:not-found" {
		t.Errorf("expected type urn:kalyx:error:not-found, got %s", prob.Type)
	}
	if prob.Title != "Resource Not Found" {
		t.Errorf("expected title 'Resource Not Found', got '%s'", prob.Title)
	}
	if prob.Detail != "Contract not found" {
		t.Errorf("expected detail 'Contract not found', got '%s'", prob.Detail)
	}
	if len(prob.InvalidParams) != 0 {
		t.Errorf("expected 0 invalid params, got %d", len(prob.InvalidParams))
	}
}

func TestDefaultTypeForStatus(t *testing.T) {
	statuses := []int{
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusMethodNotAllowed,
		http.StatusConflict,
		http.StatusUnprocessableEntity,
		http.StatusInternalServerError,
		http.StatusTeapot,
	}

	for _, s := range statuses {
		u := DefaultTypeForStatus(s)
		if u == "" {
			t.Errorf("expected non-empty type URN for status %d", s)
		}
	}
}
