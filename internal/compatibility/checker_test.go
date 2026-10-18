package compatibility

import (
	"testing"
)

func TestCheckCompatibility_AddedOptionalField(t *testing.T) {
	oldSchema := `{
		"properties": {
			"order_id": {"type": "string"}
		},
		"required": ["order_id"]
	}`

	newSchema := `{
		"properties": {
			"order_id": {"type": "string"},
			"notes": {"type": "string"}
		},
		"required": ["order_id"]
	}`

	res, err := CheckCompatibility(oldSchema, newSchema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !res.IsCompatible {
		t.Errorf("expected compatible, got incompatible: %v", res.Reasons)
	}
	if res.ChangeType != ChangeMinor {
		t.Errorf("expected MINOR change, got %v", res.ChangeType)
	}
}

func TestCheckCompatibility_AddedRequiredField(t *testing.T) {
	oldSchema := `{
		"properties": {
			"order_id": {"type": "string"}
		},
		"required": ["order_id"]
	}`

	newSchema := `{
		"properties": {
			"order_id": {"type": "string"},
			"user_id": {"type": "string"}
		},
		"required": ["order_id", "user_id"]
	}`

	res, err := CheckCompatibility(oldSchema, newSchema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.IsCompatible {
		t.Errorf("expected incompatible breaking change when adding required field")
	}
	if res.ChangeType != ChangeMajor {
		t.Errorf("expected MAJOR change, got %v", res.ChangeType)
	}
}

func TestCheckCompatibility_RemovedField(t *testing.T) {
	oldSchema := `{
		"properties": {
			"order_id": {"type": "string"},
			"currency": {"type": "string"}
		},
		"required": ["order_id"]
	}`

	newSchema := `{
		"properties": {
			"order_id": {"type": "string"}
		},
		"required": ["order_id"]
	}`

	res, err := CheckCompatibility(oldSchema, newSchema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.IsCompatible {
		t.Errorf("expected incompatible breaking change when removing a field")
	}
	if res.ChangeType != ChangeMajor {
		t.Errorf("expected MAJOR change, got %v", res.ChangeType)
	}
}
