package datalake

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// DeltaGenerator implements Generator for Delta Lake DDL.
type DeltaGenerator struct{}

// Format returns the format identifier for Delta Lake.
func (d *DeltaGenerator) Format() string {
	return FormatDelta
}

// GenerateDDL generates a Delta Lake CREATE TABLE statement from JSON Schema Draft 2020-12.
func (d *DeltaGenerator) GenerateDDL(tableName string, schemaJSON string, partitionKeys []string) (string, error) {
	return GenerateDeltaDDL(tableName, schemaJSON, partitionKeys)
}

// GenerateDeltaDDL produces a Delta Lake CREATE TABLE statement ('USING delta') from a JSON Schema.
func GenerateDeltaDDL(tableName string, schemaJSON string, partitionKeys []string) (string, error) {
	var doc SchemaDocument
	if err := json.Unmarshal([]byte(schemaJSON), &doc); err != nil {
		return "", fmt.Errorf("failed to parse schema for Delta DDL: %w", err)
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
		sqlType := mapJSONTypeToDelta(prop)
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

	ddl := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n%s\n) USING delta", tableName, strings.Join(columns, ",\n"))

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

// mapJSONTypeToDelta maps JSON Schema Draft 2020-12 data types to Delta Lake SQL types.
func mapJSONTypeToDelta(prop SchemaProperty) string {
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
		if prop.Format == "int32" {
			return "INT"
		}
		return "BIGINT"
	case "number":
		if prop.Format == "float" {
			return "FLOAT"
		}
		return "DOUBLE"
	case "boolean":
		return "BOOLEAN"
	case "array":
		if prop.Items != nil {
			return fmt.Sprintf("ARRAY<%s>", mapJSONTypeToDelta(*prop.Items))
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
				subCols = append(subCols, fmt.Sprintf("%s: %s", name, mapJSONTypeToDelta(subProp)))
			}
			return fmt.Sprintf("STRUCT<%s>", strings.Join(subCols, ", "))
		}
		return "STRUCT<raw: STRING>"
	default:
		return "STRING"
	}
}
