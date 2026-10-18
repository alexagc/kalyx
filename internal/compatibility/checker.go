package compatibility

import (
	"encoding/json"
	"fmt"
)

// ChangeType defines the SemVer impact of a schema change.
type ChangeType string

const (
	ChangeMajor ChangeType = "MAJOR" // Breaking change
	ChangeMinor ChangeType = "MINOR" // Backward-compatible addition
	ChangePatch ChangeType = "PATCH" // Non-functional or metadata change
)

// CompatibilityResult holds the diagnosis of a schema comparison.
type CompatibilityResult struct {
	IsCompatible bool       `json:"is_compatible"`
	ChangeType   ChangeType `json:"change_type"`
	Reasons      []string   `json:"reasons"`
}

// SimpleSchema represents the essential fields of a JSON Schema object for comparison.
type SimpleSchema struct {
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required"`
}

// CheckCompatibility compares old and new JSON Schema strings and determines SemVer impact.
func CheckCompatibility(oldSchemaJSON, newSchemaJSON string) (*CompatibilityResult, error) {
	if oldSchemaJSON == "" {
		// New schema from scratch
		return &CompatibilityResult{
			IsCompatible: true,
			ChangeType:   ChangeMajor,
			Reasons:      []string{"Initial schema creation"},
		}, nil
	}

	var oldSchema, newSchema SimpleSchema
	if err := json.Unmarshal([]byte(oldSchemaJSON), &oldSchema); err != nil {
		return nil, fmt.Errorf("failed to parse existing schema: %w", err)
	}
	if err := json.Unmarshal([]byte(newSchemaJSON), &newSchema); err != nil {
		return nil, fmt.Errorf("failed to parse proposed schema: %w", err)
	}

	result := &CompatibilityResult{
		IsCompatible: true,
		ChangeType:   ChangePatch,
		Reasons:      make([]string, 0),
	}

	// 1. Check for removed properties (Breaking)
	for propName := range oldSchema.Properties {
		if _, exists := newSchema.Properties[propName]; !exists {
			result.IsCompatible = false
			result.ChangeType = ChangeMajor
			result.Reasons = append(result.Reasons, fmt.Sprintf("Removed existing property: '%s'", propName))
		}
	}

	// 2. Check for newly required properties (Breaking for consumers/producers)
	oldRequiredMap := make(map[string]bool)
	for _, req := range oldSchema.Required {
		oldRequiredMap[req] = true
	}
	for _, req := range newSchema.Required {
		if !oldRequiredMap[req] {
			result.IsCompatible = false
			result.ChangeType = ChangeMajor
			result.Reasons = append(result.Reasons, fmt.Sprintf("Added new required property: '%s'", req))
		}
	}

	// 3. Check for newly added optional properties (Compatible / Minor)
	if result.IsCompatible {
		hasNewProps := false
		for propName := range newSchema.Properties {
			if _, exists := oldSchema.Properties[propName]; !exists {
				hasNewProps = true
				result.Reasons = append(result.Reasons, fmt.Sprintf("Added optional property: '%s'", propName))
			}
		}
		if hasNewProps {
			result.ChangeType = ChangeMinor
		}
	}

	return result, nil
}
