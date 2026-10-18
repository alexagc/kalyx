package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/alexagc/kalyx/internal/cli/codegen"
	"github.com/alexagc/kalyx/internal/core"
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
