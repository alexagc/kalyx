package datalake

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Supported data lake DDL formats.
const (
	FormatIceberg = "iceberg"
	FormatDelta   = "delta"
)

// SchemaProperty defines type and constraints in JSON Schema Draft 2020-12.
type SchemaProperty struct {
	Type        string                    `json:"type"`
	Format      string                    `json:"format,omitempty"`
	Description string                    `json:"description,omitempty"`
	Properties  map[string]SchemaProperty `json:"properties,omitempty"`
	Items       *SchemaProperty           `json:"items,omitempty"`
}

// SchemaDocument represents a JSON Schema document containing properties and required fields.
type SchemaDocument struct {
	Title       string                    `json:"title"`
	Description string                    `json:"description,omitempty"`
	Properties  map[string]SchemaProperty `json:"properties"`
	Required    []string                  `json:"required"`
}

// Generator defines the unified interface for Data Lake DDL generators.
type Generator interface {
	Format() string
	GenerateDDL(tableName string, schemaJSON string, partitionKeys []string) (string, error)
}

// IcebergGenerator produces Apache Iceberg CREATE TABLE DDL statements.
type IcebergGenerator struct{}

// Format returns the format identifier for Apache Iceberg.
func (g *IcebergGenerator) Format() string {
	return FormatIceberg
}

// GenerateDDL generates an Apache Iceberg DDL statement from JSON Schema Draft 2020-12.
func (g *IcebergGenerator) GenerateDDL(tableName string, schemaJSON string, partitionKeys []string) (string, error) {
	return GenerateIcebergDDL(tableName, schemaJSON, partitionKeys)
}

// GetGenerator retrieves the appropriate Generator for the specified format.
func GetGenerator(format string) (Generator, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case FormatIceberg, "":
		return &IcebergGenerator{}, nil
	case FormatDelta, "deltalake":
		return &DeltaGenerator{}, nil
	default:
		return nil, fmt.Errorf("unsupported datalake format: %s (supported: iceberg, delta)", format)
	}
}

// GenerateIcebergDDL produces an Apache Iceberg CREATE TABLE statement from a JSON Schema.
func GenerateIcebergDDL(tableName string, schemaJSON string, partitionKeys []string) (string, error) {
	var doc SchemaDocument
	if err := json.Unmarshal([]byte(schemaJSON), &doc); err != nil {
		return "", fmt.Errorf("failed to parse schema for DDL: %w", err)
	}

	requiredMap := make(map[string]bool)
	for _, req := range doc.Required {
		requiredMap[req] = true
	}

	var columns []string
	// Standard CloudEvents envelope columns
	columns = append(columns, "  id STRING NOT NULL COMMENT 'CloudEvent ID'")
	columns = append(columns, "  source STRING NOT NULL COMMENT 'CloudEvent Source'")
	columns = append(columns, "  type STRING NOT NULL COMMENT 'CloudEvent Type'")
	columns = append(columns, "  time TIMESTAMP NOT NULL COMMENT 'CloudEvent Timestamp'")
	columns = append(columns, "  datacontenttype STRING COMMENT 'CloudEvent Content Type'")
	columns = append(columns, "  dataschema STRING COMMENT 'CloudEvent Schema URL'")

	// Sort properties for deterministic DDL output
	propNames := make([]string, 0, len(doc.Properties))
	for name := range doc.Properties {
		propNames = append(propNames, name)
	}
	sort.Strings(propNames)

	// Payload data columns translated from JSON Schema
	for _, propName := range propNames {
		prop := doc.Properties[propName]
		sqlType := mapJSONTypeToIceberg(prop)
		nullable := ""
		if requiredMap[propName] {
			nullable = " NOT NULL"
		}
		comment := ""
		if prop.Description != "" {
			comment = fmt.Sprintf(" COMMENT '%s'", strings.ReplaceAll(prop.Description, "'", "\\'"))
		}
		columns = append(columns, fmt.Sprintf("  data_%s %s%s%s", propName, sqlType, nullable, comment))
	}

	ddl := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n%s\n) USING iceberg", tableName, strings.Join(columns, ",\n"))

	if len(partitionKeys) > 0 {
		var parts []string
		for _, pk := range partitionKeys {
			parts = append(parts, fmt.Sprintf("data_%s", pk))
		}
		ddl += fmt.Sprintf("\nPARTITIONED BY (%s);", strings.Join(parts, ", "))
	} else {
		ddl += ";"
	}

	return ddl, nil
}

func mapJSONTypeToIceberg(prop SchemaProperty) string {
	switch prop.Type {
	case "string":
		if prop.Format == "date-time" {
			return "TIMESTAMP"
		}
		if prop.Format == "date" {
			return "DATE"
		}
		return "STRING"
	case "integer":
		return "BIGINT"
	case "number":
		return "DOUBLE"
	case "boolean":
		return "BOOLEAN"
	case "array":
		if prop.Items != nil {
			return fmt.Sprintf("ARRAY<%s>", mapJSONTypeToIceberg(*prop.Items))
		}
		return "ARRAY<STRING>"
	case "object":
		if len(prop.Properties) > 0 {
			nestedNames := make([]string, 0, len(prop.Properties))
			for name := range prop.Properties {
				nestedNames = append(nestedNames, name)
			}
			sort.Strings(nestedNames)

			var subCols []string
			for _, name := range nestedNames {
				subProp := prop.Properties[name]
				subCols = append(subCols, fmt.Sprintf("%s: %s", name, mapJSONTypeToIceberg(subProp)))
			}
			return fmt.Sprintf("STRUCT<%s>", strings.Join(subCols, ", "))
		}
		return "STRING"
	default:
		return "STRING"
	}
}
