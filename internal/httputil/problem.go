package httputil

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// InvalidParam represents a single parameter validation failure in RFC 7807 problem details.
type InvalidParam struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// ProblemDetails represents an RFC 7807 Problem Details payload for HTTP APIs.
// See: https://datatracker.ietf.org/doc/html/rfc7807
type ProblemDetails struct {
	Type          string         `json:"type"`
	Title         string         `json:"title"`
	Status        int            `json:"status"`
	Detail        string         `json:"detail,omitempty"`
	Instance      string         `json:"instance,omitempty"`
	InvalidParams []InvalidParam `json:"invalid_params,omitempty"`
}

// DefaultTypeForStatus returns a standardized URN identifier for common HTTP statuses.
func DefaultTypeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "urn:kalyx:error:bad-request"
	case http.StatusUnauthorized:
		return "urn:kalyx:error:unauthorized"
	case http.StatusForbidden:
		return "urn:kalyx:error:forbidden"
	case http.StatusNotFound:
		return "urn:kalyx:error:not-found"
	case http.StatusMethodNotAllowed:
		return "urn:kalyx:error:method-not-allowed"
	case http.StatusConflict:
		return "urn:kalyx:error:conflict"
	case http.StatusUnprocessableEntity:
		return "urn:kalyx:error:validation-failed"
	case http.StatusInternalServerError:
		return "urn:kalyx:error:internal"
	default:
		return "about:blank"
	}
}

// NewProblem creates a new ProblemDetails struct with appropriate defaults.
func NewProblem(status int, title, detail string, invalidParams []InvalidParam) ProblemDetails {
	if title == "" {
		title = http.StatusText(status)
	}
	return ProblemDetails{
		Type:          DefaultTypeForStatus(status),
		Title:         title,
		Status:        status,
		Detail:        detail,
		InvalidParams: invalidParams,
	}
}

// WriteProblem writes an RFC 7807 Problem Details JSON response with application/problem+json content type.
func WriteProblem(w http.ResponseWriter, status int, title, detail string, invalidParams []InvalidParam) {
	prob := NewProblem(status, title, detail, invalidParams)
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(prob); err != nil {
		// Fallback in case encoding fails
		http.Error(w, fmt.Sprintf(`{"status":%d,"title":"%s"}`, status, title), status)
	}
}

// WriteProblemSimple is a convenience helper for RFC 7807 responses without invalid parameter details.
func WriteProblemSimple(w http.ResponseWriter, status int, title, detail string) {
	WriteProblem(w, status, title, detail, nil)
}
