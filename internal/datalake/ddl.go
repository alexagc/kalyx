package datalake

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SchemaProperty defines type and constraints in JSON Schema.
type SchemaProperty struct {
	Type        string `json:"type"`
	Format      string `json:"format,omitempty"`
	Description string `json:"description,omitempty"`
}

type SchemaDocument struct {
	Title      string                    `json:"title"`
	Properties map[string]SchemaProperty `json:"properties"`
	Required   []string                  `json:"required"`
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

	// Payload data columns translated from JSON Schema
	for propName, prop := range doc.Properties {
		sqlType := mapJSONTypeToIceberg(prop.Type, prop.Format)
		nullable := ""
		if requiredMap[propName] {
			nullable = " NOT NULL"
		}
		comment := ""
		if prop.Description != "" {
			comment = fmt.Sprintf(" COMMENT '%s'", prop.Description)
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

func mapJSONTypeToIceberg(jsonType, format string) string {
	switch jsonType {
	case "string":
		if format == "date-time" {
			return "TIMESTAMP"
		}
		if format == "date" {
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
		return "ARRAY<STRING>"
	case "object":
		return "STRING" // Or STRUCT in advanced cases
	default:
		return "STRING"
	}
}
