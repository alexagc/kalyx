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
	if !strings.Contains(ddl, "data_order_id STRING NOT NULL") {
		t.Errorf("order_id should be STRING NOT NULL")
	}
	if !strings.Contains(ddl, "data_amount DOUBLE NOT NULL") {
		t.Errorf("amount should be DOUBLE NOT NULL")
	}
	if !strings.Contains(ddl, "data_created_at TIMESTAMP") {
		t.Errorf("created_at should be TIMESTAMP")
	}
	if !strings.Contains(ddl, "PARTITIONED BY (data_created_at)") {
		t.Errorf("missing partition clause")
	}
}
