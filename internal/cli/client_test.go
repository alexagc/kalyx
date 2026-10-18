package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexagc/kalyx/internal/core"
)

func TestClient_GetMe(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/me", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer valid-token" {
			http.Error(w, `{"status":401,"title":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(core.UserContext{
			Subject:   "usr_123",
			Email:     "developer@kalyx.io",
			Teams:     []string{"Core", "Platform"},
			Roles:     []string{"member", "lead"},
			IsMachine: false,
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx := context.Background()

	// 1. Success case
	client := NewClient(ts.URL, "valid-token")
	user, err := client.GetMe(ctx)
	if err != nil {
		t.Fatalf("GetMe failed: %v", err)
	}
	if user.Email != "developer@kalyx.io" {
		t.Errorf("expected email 'developer@kalyx.io', got '%s'", user.Email)
	}
	if len(user.Teams) != 2 || user.Teams[0] != "Core" {
		t.Errorf("unexpected teams: %v", user.Teams)
	}

	// 2. Unauthorized case
	clientBad := NewClient(ts.URL, "invalid-token")
	_, errBad := clientBad.GetMe(ctx)
	if errBad == nil {
		t.Fatalf("expected error for invalid token, got nil")
	}
}

func TestClient_ValidateEvent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/validate", func(w http.ResponseWriter, r *http.Request) {
		var wrapper struct {
			Schema string          `json:"schema"`
			Event  json.RawMessage `json:"event"`
		}
		_ = json.NewDecoder(r.Body).Decode(&wrapper)

		var ev struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(wrapper.Event, &ev)

		if ev.ID == "valid-event" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"valid":    true,
				"event_id": "com.company.order",
				"semver":   "1.0.0",
				"status":   "valid",
			})
			return
		}

		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type":   "urn:kalyx:error:validation-failed",
			"title":  "Validation Failed",
			"status": 422,
			"invalid_params": []map[string]string{
				{"name": "data.order_id", "reason": "field is required"},
			},
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx := context.Background()
	client := NewClient(ts.URL, "")

	// 1. Valid event
	validBytes := []byte(`{"id":"valid-event","data":{"order_id":"123"}}`)
	resValid, err := client.ValidateEvent(ctx, validBytes, `{"type":"object"}`)
	if err != nil {
		t.Fatalf("ValidateEvent failed: %v", err)
	}
	if !resValid.Valid {
		t.Errorf("expected valid event result, got invalid: %v", resValid.Errors)
	}

	// 2. Invalid event
	invalidBytes := []byte(`{"id":"invalid-event","data":{}}`)
	resInvalid, err := client.ValidateEvent(ctx, invalidBytes, `{"type":"object"}`)
	if err != nil {
		t.Fatalf("ValidateEvent returned unexpected transport error: %v", err)
	}
	if resInvalid.Valid {
		t.Errorf("expected invalid event result, got valid")
	}
	if len(resInvalid.Errors) != 1 || resInvalid.Errors[0] != "data.order_id: field is required" {
		t.Errorf("unexpected validation errors: %v", resInvalid.Errors)
	}
}

func TestClient_CheckCompatibility(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/compatibility/check", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			OldSchema string `json:"old_schema"`
			NewSchema string `json:"new_schema"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		if req.OldSchema == "old_ok" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"is_compatible": true,
				"change_type":   "MINOR",
				"reasons":       []string{"Added optional field"},
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"is_compatible": false,
			"change_type":   "MAJOR",
			"reasons":       []string{"Removed required field"},
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx := context.Background()
	client := NewClient(ts.URL, "")

	// 1. Compatible
	resOk, err := client.CheckCompatibility(ctx, "old_ok", "new_ok")
	if err != nil {
		t.Fatalf("CheckCompatibility failed: %v", err)
	}
	if !resOk.IsCompatible || resOk.ChangeType != "MINOR" {
		t.Errorf("expected compatible MINOR change, got %v", resOk)
	}

	// 2. Breaking
	resBreaking, err := client.CheckCompatibility(ctx, "old_breaking", "new_breaking")
	if err != nil {
		t.Fatalf("CheckCompatibility failed: %v", err)
	}
	if resBreaking.IsCompatible || resBreaking.ChangeType != "MAJOR" {
		t.Errorf("expected breaking MAJOR change, got %v", resBreaking)
	}
}

func TestClient_CreatePAT(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/tokens", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name          string `json:"name"`
			ExpiresInDays int    `json:"expires_in_days"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":          "pat_123",
			"name":        req.Name,
			"token":       "kalyx_sec_generated_token_xyz",
			"owner_email": "ci@company.com",
			"team":        "DevOps",
			"created_at":  time.Now().UTC(),
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx := context.Background()
	client := NewClient(ts.URL, "auth-token")

	resp, err := client.CreatePAT(ctx, "github-actions", 60)
	if err != nil {
		t.Fatalf("CreatePAT failed: %v", err)
	}
	if resp.Token != "kalyx_sec_generated_token_xyz" {
		t.Errorf("expected raw token 'kalyx_sec_generated_token_xyz', got '%s'", resp.Token)
	}
	if resp.Name != "github-actions" {
		t.Errorf("expected name 'github-actions', got '%s'", resp.Name)
	}
}

func TestClient_CreateVersion(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/events/ev_order/versions", func(w http.ResponseWriter, r *http.Request) {
		var v core.EventVersion
		_ = json.NewDecoder(r.Body).Decode(&v)
		if v.SemVer != "1.0.0" {
			http.Error(w, "bad semver", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx := context.Background()
	client := NewClient(ts.URL, "")

	err := client.CreateVersion(ctx, "ev_order", core.EventVersion{
		SemVer:     "1.0.0",
		JSONSchema: `{"type":"object"}`,
	})
	if err != nil {
		t.Fatalf("CreateVersion failed: %v", err)
	}
}
