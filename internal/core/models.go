package core

import "time"

// EventStatus defines the lifecycle status of an event version.
type EventStatus string

const (
	StatusDraft      EventStatus = "draft"
	StatusInReview   EventStatus = "in_review"
	StatusApproved   EventStatus = "approved"
	StatusDeprecated EventStatus = "deprecated"
)

// DataLakeMetadata defines extensions for Data Lake integration.
type DataLakeMetadata struct {
	TargetTable string   `json:"target_table,omitempty"`
	PartitionBy []string `json:"partition_by,omitempty"`
	PIIFields   []string `json:"pii_fields,omitempty"`
}

// EventDefinition represents a distinct business event entity.
type EventDefinition struct {
	ID          string    `json:"id"`
	Domain      string    `json:"domain"`
	Name        string    `json:"name"`
	Type        string    `json:"type"` // e.g., com.company.orders.completed
	Description string    `json:"description"`
	OwnerTeam   string    `json:"owner_team"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// EventVersion represents an immutable version of an event contract.
type EventVersion struct {
	ID               string           `json:"id"`
	EventID          string           `json:"event_id"`
	SemVer           string           `json:"semver"` // e.g. "1.0.0"
	Status           EventStatus      `json:"status"`
	JSONSchema       string           `json:"json_schema"` // Draft 2020-12 JSON Schema
	DataLakeMetadata DataLakeMetadata `json:"datalake_metadata"`
	CreatedAt        time.Time        `json:"created_at"`
	CreatedBy        string           `json:"created_by"`
	ApprovedBy       string           `json:"approved_by,omitempty"`
	ApprovedAt       *time.Time       `json:"approved_at,omitempty"`
}

// ChangeProposal represents a review request to update or create an event schema.
type ChangeProposal struct {
	ID             string      `json:"id"`
	EventID        string      `json:"event_id"`
	TargetSemVer   string      `json:"target_semver"`
	ProposedSchema string      `json:"proposed_schema"`
	ProposedBy     string      `json:"proposed_by"`
	Status         EventStatus `json:"status"`
	ReviewNotes    string      `json:"review_notes,omitempty"`
	IsBreaking     bool        `json:"is_breaking"`
	BreakingReason string      `json:"breaking_reason,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
}

// PATToken represents a hashed Personal Access Token for CI/CD and service authentication.
type PATToken struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	TokenHash  string     `json:"-"` // SHA-256 hash, never exposed in JSON
	OwnerEmail string     `json:"owner_email"`
	Team       string     `json:"team"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

