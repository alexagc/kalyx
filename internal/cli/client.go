package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/alexagc/kalyx/internal/cli/codegen"
	"github.com/alexagc/kalyx/internal/core"
	"github.com/alexagc/kalyx/internal/httputil"
)

// Client handles HTTP API communication between kalyxctl and the Kalyx server.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// NewClient creates a new API client for interacting with Kalyx server.
func NewClient(baseURL, token string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) newRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Request, error) {
	url := c.BaseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	return req, nil
}

// ListEvents retrieves all event definitions registered on the server.
func (c *Client) ListEvents(ctx context.Context) ([]core.EventDefinition, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/events", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Kalyx server at %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var events []core.EventDefinition
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return nil, fmt.Errorf("failed to decode events response: %w", err)
	}
	return events, nil
}

// ListVersions retrieves all contract versions for a given event ID.
func (c *Client) ListVersions(ctx context.Context, eventID string) ([]core.EventVersion, error) {
	endpoint := fmt.Sprintf("/api/v1/events/%s/versions", eventID)
	req, err := c.newRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var versions []core.EventVersion
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return nil, fmt.Errorf("failed to decode versions response: %w", err)
	}
	return versions, nil
}

// FetchContract resolves an event by its identifier (Type, ID, or Name) and returns the EventContract for codegen.
func (c *Client) FetchContract(ctx context.Context, identifier string) (*codegen.EventContract, error) {
	events, err := c.ListEvents(ctx)
	if err != nil {
		return nil, err
	}

	var targetEvent *core.EventDefinition
	for _, ev := range events {
		if ev.Type == identifier || ev.ID == identifier || ev.Name == identifier {
			evCopy := ev
			targetEvent = &evCopy
			break
		}
	}

	if targetEvent == nil {
		return nil, fmt.Errorf("event contract '%s' not found on server", identifier)
	}

	versions, err := c.ListVersions(ctx, targetEvent.ID)
	if err != nil {
		return nil, err
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no contract versions found for event '%s'", targetEvent.ID)
	}

	// Pick the latest approved version, or latest overall
	var selectedVersion *core.EventVersion
	for _, v := range versions {
		if v.Status == core.StatusApproved {
			vCopy := v
			selectedVersion = &vCopy
			break
		}
	}
	if selectedVersion == nil {
		vCopy := versions[len(versions)-1]
		selectedVersion = &vCopy
	}

	schemaDef, err := codegen.ParseJSONSchema(selectedVersion.JSONSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON schema for event '%s': %w", targetEvent.Type, err)
	}

	isDeprecated := selectedVersion.Status == core.StatusDeprecated
	sunsetDate := ""
	if isDeprecated {
		sunsetDate = time.Now().AddDate(0, 6, 0).UTC().Format(time.RFC3339)
	}

	return &codegen.EventContract{
		ID:          targetEvent.ID,
		Name:        targetEvent.Name,
		Type:        targetEvent.Type,
		SemVer:      selectedVersion.SemVer,
		Description: targetEvent.Description,
		Deprecated:  isDeprecated,
		SunsetDate:  sunsetDate,
		Schema:      schemaDef,
	}, nil
}

// ValidationResult summarizes the outcome of event validation against a contract schema.
type ValidationResult struct {
	Valid   bool     `json:"valid"`
	EventID string   `json:"event_id,omitempty"`
	SemVer  string   `json:"semver,omitempty"`
	Status  string   `json:"status,omitempty"`
	Errors  []string `json:"errors,omitempty"`
}

// CompatibilityResult holds the diagnosis of a schema comparison.
type CompatibilityResult struct {
	IsCompatible bool     `json:"is_compatible"`
	ChangeType   string   `json:"change_type"`
	Reasons      []string `json:"reasons"`
}

// PATCreateResponse contains the generated Personal Access Token and its metadata.
type PATCreateResponse struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Token      string     `json:"token"`
	OwnerEmail string     `json:"owner_email"`
	Team       string     `json:"team"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// GetMe calls GET /api/v1/me with Bearer token to fetch the authenticated user's context.
func (c *Client) GetMe(ctx context.Context) (*core.UserContext, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/me", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Kalyx server at %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("unauthorized: invalid or expired token")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var user core.UserContext
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("failed to decode user response: %w", err)
	}
	return &user, nil
}

// ValidateEvent calls POST /api/v1/validate with eventBytes and optional schema to validate a CloudEvent.
func (c *Client) ValidateEvent(ctx context.Context, eventBytes []byte, schema string) (*ValidationResult, error) {
	var body io.Reader
	if strings.TrimSpace(schema) != "" {
		wrapper := struct {
			Schema string          `json:"schema"`
			Event  json.RawMessage `json:"event"`
		}{
			Schema: schema,
			Event:  json.RawMessage(eventBytes),
		}
		data, err := json.Marshal(wrapper)
		if err != nil {
			return nil, fmt.Errorf("failed to encode validation payload: %w", err)
		}
		body = bytes.NewReader(data)
	} else {
		body = bytes.NewReader(eventBytes)
	}

	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/validate", body)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to validate event at %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		var result ValidationResult
		if err := json.Unmarshal(respBytes, &result); err != nil {
			return nil, fmt.Errorf("failed to parse validation response: %w", err)
		}
		result.Valid = true
		return &result, nil
	}

	if resp.StatusCode == http.StatusUnprocessableEntity {
		var prob httputil.ProblemDetails
		if err := json.Unmarshal(respBytes, &prob); err == nil {
			var errMsgs []string
			for _, p := range prob.InvalidParams {
				errMsgs = append(errMsgs, fmt.Sprintf("%s: %s", p.Name, p.Reason))
			}
			if len(errMsgs) == 0 && prob.Detail != "" {
				errMsgs = append(errMsgs, prob.Detail)
			}
			return &ValidationResult{
				Valid:  false,
				Errors: errMsgs,
			}, nil
		}
		return &ValidationResult{
			Valid:  false,
			Errors: []string{strings.TrimSpace(string(respBytes))},
		}, nil
	}

	return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
}

// CheckCompatibility calls POST /api/v1/compatibility/check to verify schema compatibility.
func (c *Client) CheckCompatibility(ctx context.Context, oldSchema, newSchema string) (*CompatibilityResult, error) {
	reqPayload := struct {
		OldSchema string `json:"old_schema"`
		NewSchema string `json:"new_schema"`
	}{
		OldSchema: oldSchema,
		NewSchema: newSchema,
	}

	data, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal compatibility request: %w", err)
	}

	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/compatibility/check", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to check compatibility at %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	var res CompatibilityResult
	if err := json.Unmarshal(respBytes, &res); err != nil {
		return nil, fmt.Errorf("failed to decode compatibility response: %w", err)
	}
	return &res, nil
}

// CreatePAT calls POST /api/v1/tokens to generate a new Personal Access Token.
func (c *Client) CreatePAT(ctx context.Context, name string, expiresInDays int) (*PATCreateResponse, error) {
	reqPayload := struct {
		Name          string `json:"name"`
		ExpiresInDays int    `json:"expires_in_days"`
	}{
		Name:          name,
		ExpiresInDays: expiresInDays,
	}

	data, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal token request: %w", err)
	}

	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/tokens", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create PAT at %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	var patResp PATCreateResponse
	if err := json.Unmarshal(respBytes, &patResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}
	return &patResp, nil
}

// CreateVersion calls POST /api/v1/events/{id}/versions to publish a new contract version.
func (c *Client) CreateVersion(ctx context.Context, eventID string, version core.EventVersion) error {
	data, err := json.Marshal(version)
	if err != nil {
		return fmt.Errorf("failed to marshal version: %w", err)
	}

	endpoint := fmt.Sprintf("/api/v1/events/%s/versions", eventID)
	req, err := c.newRequest(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create version at %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}

