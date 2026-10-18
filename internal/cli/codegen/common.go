package codegen

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// PropertySchema represents an individual property extracted from JSON Schema.
type PropertySchema struct {
	Name        string
	Type        string // string, number, integer, boolean, array, object
	Description string
	Format      string
	ItemType    string // if Type == "array"
	Required    bool
}

// JSONSchemaDefinition contains structured data parsed from Draft 2020-12 JSON Schema.
type JSONSchemaDefinition struct {
	Title       string
	Description string
	Type        string
	Properties  []PropertySchema
	Required    []string
}

// EventContract encapsulates all metadata required for generating client SDK code.
type EventContract struct {
	ID          string
	Name        string // e.g. "OrderCompleted"
	Type        string // e.g. "com.company.orders.completed.v1"
	SemVer      string // e.g. "1.0.0"
	Description string
	Deprecated  bool
	SunsetDate  string
	Schema      JSONSchemaDefinition
}

type rawProperty struct {
	Type        string       `json:"type"`
	Description string       `json:"description"`
	Format      string       `json:"format"`
	Items       *rawProperty `json:"items"`
}

type rawSchema struct {
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Type        string                 `json:"type"`
	Properties  map[string]rawProperty `json:"properties"`
	Required    []string               `json:"required"`
}

// ParseJSONSchema parses a JSON Schema Draft 2020-12 string into a JSONSchemaDefinition.
func ParseJSONSchema(rawJSON string) (JSONSchemaDefinition, error) {
	var raw rawSchema
	if err := json.Unmarshal([]byte(rawJSON), &raw); err != nil {
		return JSONSchemaDefinition{}, fmt.Errorf("failed to parse JSON schema: %w", err)
	}

	reqMap := make(map[string]bool)
	for _, r := range raw.Required {
		reqMap[r] = true
	}

	props := make([]PropertySchema, 0, len(raw.Properties))
	for name, prop := range raw.Properties {
		itemType := "any"
		if prop.Items != nil && prop.Items.Type != "" {
			itemType = prop.Items.Type
		}
		props = append(props, PropertySchema{
			Name:        name,
			Type:        prop.Type,
			Description: prop.Description,
			Format:      prop.Format,
			ItemType:    itemType,
			Required:    reqMap[name],
		})
	}

	// Sort properties deterministically by name
	sort.Slice(props, func(i, j int) bool {
		return props[i].Name < props[j].Name
	})

	return JSONSchemaDefinition{
		Title:       raw.Title,
		Description: raw.Description,
		Type:        raw.Type,
		Properties:  props,
		Required:    raw.Required,
	}, nil
}

// DeriveEventName resolves an idiomatic PascalCase event name from Name or Type.
func DeriveEventName(name, eventType string) string {
	if name != "" {
		return ToPascalCase(name)
	}

	// Derive from eventType, e.g. "com.company.orders.completed.v1"
	cleaned := strings.TrimSpace(eventType)
	parts := strings.Split(cleaned, ".")
	if len(parts) == 0 {
		return "Event"
	}

	// Remove trailing version identifier (e.g., "v1", "v1_0_0")
	last := parts[len(parts)-1]
	if strings.HasPrefix(strings.ToLower(last), "v") && len(parts) > 1 {
		parts = parts[:len(parts)-1]
	}

	if len(parts) >= 2 {
		// Use last two parts (e.g., "orders", "completed") -> "OrdersCompleted"
		return ToPascalCase(parts[len(parts)-2] + "_" + parts[len(parts)-1])
	}

	return ToPascalCase(parts[len(parts)-1])
}

// ToPascalCase converts strings like "order_id", "orderCompleted", "order-id" to "OrderId".
func ToPascalCase(s string) string {
	if s == "" {
		return ""
	}
	var sb strings.Builder
	capitalizeNext := true

	for i, r := range s {
		if r == '_' || r == '-' || r == '.' || unicode.IsSpace(r) {
			capitalizeNext = true
			continue
		}
		if capitalizeNext {
			sb.WriteRune(unicode.ToUpper(r))
			capitalizeNext = false
		} else if i > 0 && unicode.IsUpper(r) && (i+1 < len(s) && unicode.IsLower(rune(s[i+1]))) {
			sb.WriteRune(r)
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// ToCamelCase converts strings like "OrderCompleted", "order_id" to "orderCompleted" or "orderId".
func ToCamelCase(s string) string {
	pascal := ToPascalCase(s)
	if pascal == "" {
		return ""
	}
	r := []rune(pascal)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// ToSnakeCase converts PascalCase or camelCase to snake_case.
func ToSnakeCase(s string) string {
	var sb strings.Builder
	for i, r := range s {
		if r == '-' || r == '.' || unicode.IsSpace(r) {
			sb.WriteRune('_')
			continue
		}
		if unicode.IsUpper(r) {
			if i > 0 && s[i-1] != '_' {
				sb.WriteRune('_')
			}
			sb.WriteRune(unicode.ToLower(r))
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
