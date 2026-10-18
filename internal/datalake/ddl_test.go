package datalake

import (
	"strings"
	"testing"
)

func TestGenerateIcebergDDL(t *testing.T) {
	schemaJSON := `{
		"title": "OrderCompleted",
		"properties": {
			"order_id": {"type": "string", "description": "Unique identifier of order"},
			"amount": {"type": "number"},
			"created_at": {"type": "string", "format": "date-time"}
		},
		"required": ["order_id", "amount"]
	}`

	ddl, err := GenerateIcebergDDL("gold_orders", schemaJSON, []string{"created_at"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(ddl, "CREATE TABLE IF NOT EXISTS gold_orders") {
		t.Errorf("missing create table statement")
	}
	if !strings.Contains(ddl, "USING iceberg") {
		t.Errorf("expected USING iceberg")
	}
	if !strings.Contains(ddl, "data_order_id STRING NOT NULL") {
		t.Errorf("order_id should be STRING NOT NULL")
	}
	if !strings.Contains(ddl, "data_amount DOUBLE NOT NULL") {
		t.Errorf("amount should be DOUBLE NOT NULL")
	}
	if !strings.Contains(ddl, "data_created_at TIMESTAMP") {
		t.Errorf("created_at should be TIMESTAMP")
	}
	if !strings.Contains(ddl, "PARTITIONED BY (data_created_at);") {
		t.Errorf("missing partition clause")
	}
}

func TestGenerateDeltaDDL(t *testing.T) {
	schemaJSON := `{
		"title": "PaymentProcessed",
		"properties": {
			"payment_id": {"type": "string", "description": "Transaction ID"},
			"amount": {"type": "number", "format": "float"},
			"attempts": {"type": "integer", "format": "int32"},
			"total_cents": {"type": "integer"},
			"is_settled": {"type": "boolean"},
			"processed_at": {"type": "string", "format": "date-time"},
			"tags": {"type": "array", "items": {"type": "string"}},
			"customer": {
				"type": "object",
				"properties": {
					"id": {"type": "string"},
					"tier": {"type": "string"}
				}
			}
		},
		"required": ["payment_id", "amount", "processed_at"]
	}`

	ddl, err := GenerateDeltaDDL("delta_payments", schemaJSON, []string{"processed_at"})
	if err != nil {
		t.Fatalf("unexpected error generating delta DDL: %v", err)
	}

	if !strings.Contains(ddl, "CREATE TABLE IF NOT EXISTS delta_payments") {
		t.Errorf("missing create table statement")
	}
	if !strings.Contains(ddl, "USING delta") {
		t.Errorf("expected USING delta, got:\n%s", ddl)
	}
	if !strings.Contains(ddl, "data_payment_id STRING NOT NULL") {
		t.Errorf("missing data_payment_id STRING NOT NULL")
	}
	if !strings.Contains(ddl, "data_amount FLOAT NOT NULL") {
		t.Errorf("missing data_amount FLOAT NOT NULL")
	}
	if !strings.Contains(ddl, "data_attempts INT") {
		t.Errorf("missing data_attempts INT")
	}
	if !strings.Contains(ddl, "data_total_cents BIGINT") {
		t.Errorf("missing data_total_cents BIGINT")
	}
	if !strings.Contains(ddl, "data_is_settled BOOLEAN") {
		t.Errorf("missing data_is_settled BOOLEAN")
	}
	if !strings.Contains(ddl, "data_processed_at TIMESTAMP NOT NULL") {
		t.Errorf("missing data_processed_at TIMESTAMP NOT NULL")
	}
	if !strings.Contains(ddl, "data_tags ARRAY<STRING>") {
		t.Errorf("missing data_tags ARRAY<STRING>")
	}
	if !strings.Contains(ddl, "data_customer STRUCT<id: STRING, tier: STRING>") {
		t.Errorf("missing data_customer STRUCT<id: STRING, tier: STRING>")
	}
	if !strings.Contains(ddl, "PARTITIONED BY (data_processed_at);") {
		t.Errorf("missing partition clause")
	}
}

func TestGetGenerator(t *testing.T) {
	tests := []struct {
		format      string
		expectedFmt string
		expectError bool
	}{
		{"iceberg", FormatIceberg, false},
		{"ICEBERG", FormatIceberg, false},
		{"", FormatIceberg, false},
		{"delta", FormatDelta, false},
		{"deltalake", FormatDelta, false},
		{"DELTA", FormatDelta, false},
		{"unknown", "", true},
	}

	for _, tt := range tests {
		gen, err := GetGenerator(tt.format)
		if tt.expectError {
			if err == nil {
				t.Errorf("expected error for format '%s', got nil", tt.format)
			}
			continue
		}
		if err != nil {
			t.Errorf("unexpected error for format '%s': %v", tt.format, err)
			continue
		}
		if gen.Format() != tt.expectedFmt {
			t.Errorf("expected generator format '%s', got '%s'", tt.expectedFmt, gen.Format())
		}
	}
}

func TestDatalake_EdgeCases(t *testing.T) {
	// Nested struct and date in Iceberg
	schemaJSON := `{
		"title": "NestedIceberg",
		"properties": {
			"invoice_date": {"type": "string", "format": "date"},
			"details": {
				"type": "object",
				"properties": {
					"item": {"type": "string"}
				}
			},
			"empty_obj": {"type": "object"}
		}
	}`

	ddl, err := GenerateIcebergDDL("nested_table", schemaJSON, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(ddl, "data_invoice_date DATE") {
		t.Errorf("expected DATE type for invoice_date")
	}
	if !strings.Contains(ddl, "data_details STRUCT<item: STRING>") {
		t.Errorf("expected STRUCT for details")
	}

	// Invalid JSON handling
	_, err = GenerateIcebergDDL("t", "{invalid", nil)
	if err == nil {
		t.Errorf("expected error on invalid JSON for Iceberg")
	}

	_, err = GenerateDeltaDDL("t", "{invalid", nil)
	if err == nil {
		t.Errorf("expected error on invalid JSON for Delta")
	}

	// Generator interface invocation
	gen, _ := GetGenerator("delta")
	deltaDDL, err := gen.GenerateDDL("delta_t", schemaJSON, []string{"invoice_date"})
	if err != nil || !strings.Contains(deltaDDL, "USING delta") {
		t.Errorf("expected successful delta generation through Generator interface")
	}
}
