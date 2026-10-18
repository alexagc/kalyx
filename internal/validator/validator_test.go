package validator

import (
	"testing"
)

const sampleSchema = `{
	"title": "OrderCompleted",
	"properties": {
		"order_id": {"type": "string", "minLength": 3},
		"amount": {"type": "number", "minimum": 0.01},
		"currency": {"type": "string", "enum": ["USD", "EUR", "GBP"]},
		"created_at": {"type": "string", "format": "date-time"},
		"customer": {
			"type": "object",
			"properties": {
				"email": {"type": "string", "format": "email"},
				"tier": {"type": "string"}
			},
			"required": ["email"]
		},
		"items": {
			"type": "array",
			"items": {
				"type": "object",
				"properties": {
					"sku": {"type": "string"},
					"quantity": {"type": "integer", "minimum": 1}
				},
				"required": ["sku", "quantity"]
			}
		}
	},
	"required": ["order_id", "amount", "currency", "customer"]
}`

func TestValidateCloudEvent_Valid(t *testing.T) {
	v := New()
	validEvent := []byte(`{
		"specversion": "1.0",
		"id": "evt-12345",
		"source": "https://service.orders.com",
		"type": "com.company.orders.completed.v1",
		"time": "2026-10-18T12:00:00Z",
		"datacontenttype": "application/json",
		"data": {
			"order_id": "ord_999",
			"amount": 149.99,
			"currency": "EUR",
			"created_at": "2026-10-18T12:00:00Z",
			"customer": {
				"email": "customer@example.com",
				"tier": "gold"
			},
			"items": [
				{"sku": "SKU-1", "quantity": 2}
			]
		}
	}`)

	res, err := v.ValidateCloudEvent(validEvent, sampleSchema)
	if err != nil {
		t.Fatalf("unexpected validation execution error: %v", err)
	}
	if !res.IsValid {
		t.Fatalf("expected valid result, got errors: %+v", res.Errors)
	}
}

func TestValidateCloudEvent_InvalidEnvelope(t *testing.T) {
	v := New()

	tests := []struct {
		name          string
		eventJSON     string
		expectedField string
	}{
		{
			name: "missing specversion",
			eventJSON: `{
				"id": "123",
				"source": "/orders",
				"type": "test",
				"data": {}
			}`,
			expectedField: "specversion",
		},
		{
			name: "invalid specversion",
			eventJSON: `{
				"specversion": "0.3",
				"id": "123",
				"source": "/orders",
				"type": "test",
				"data": {}
			}`,
			expectedField: "specversion",
		},
		{
			name: "missing id",
			eventJSON: `{
				"specversion": "1.0",
				"source": "/orders",
				"type": "test",
				"data": {}
			}`,
			expectedField: "id",
		},
		{
			name: "missing source",
			eventJSON: `{
				"specversion": "1.0",
				"id": "123",
				"type": "test",
				"data": {}
			}`,
			expectedField: "source",
		},
		{
			name: "missing type",
			eventJSON: `{
				"specversion": "1.0",
				"id": "123",
				"source": "/orders",
				"data": {}
			}`,
			expectedField: "type",
		},
		{
			name: "invalid time format",
			eventJSON: `{
				"specversion": "1.0",
				"id": "123",
				"source": "/orders",
				"type": "test",
				"time": "not-a-timestamp",
				"data": {}
			}`,
			expectedField: "time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := v.ValidateCloudEvent([]byte(tt.eventJSON), sampleSchema)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.IsValid {
				t.Errorf("expected validation to fail for %s", tt.name)
			}
			found := false
			for _, e := range res.Errors {
				if e.Field == tt.expectedField {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error on field '%s', got errors: %+v", tt.expectedField, res.Errors)
			}
		})
	}
}

func TestValidateCloudEvent_InvalidPayload(t *testing.T) {
	v := New()

	tests := []struct {
		name          string
		dataJSON      string
		expectedField string
	}{
		{
			name:          "missing required order_id",
			dataJSON:      `{"amount": 10.0, "currency": "USD", "customer": {"email": "a@b.com"}}`,
			expectedField: "data.order_id",
		},
		{
			name:          "amount below minimum",
			dataJSON:      `{"order_id": "ord_1", "amount": 0.0, "currency": "USD", "customer": {"email": "a@b.com"}}`,
			expectedField: "data.amount",
		},
		{
			name:          "invalid currency enum",
			dataJSON:      `{"order_id": "ord_1", "amount": 10.0, "currency": "JPY", "customer": {"email": "a@b.com"}}`,
			expectedField: "data.currency",
		},
		{
			name:          "invalid customer email format",
			dataJSON:      `{"order_id": "ord_1", "amount": 10.0, "currency": "USD", "customer": {"email": "not-an-email"}}`,
			expectedField: "data.customer.email",
		},
		{
			name:          "missing nested required customer email",
			dataJSON:      `{"order_id": "ord_1", "amount": 10.0, "currency": "USD", "customer": {"tier": "gold"}}`,
			expectedField: "data.customer.email",
		},
		{
			name: "invalid item quantity in array",
			dataJSON: `{
				"order_id": "ord_1",
				"amount": 10.0,
				"currency": "USD",
				"customer": {"email": "a@b.com"},
				"items": [{"sku": "SKU-1", "quantity": 0}]
			}`,
			expectedField: "data.items[0].quantity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawEvent := `{
				"specversion": "1.0",
				"id": "evt-123",
				"source": "/test",
				"type": "test.event",
				"data": ` + tt.dataJSON + `
			}`

			res, err := v.ValidateCloudEvent([]byte(rawEvent), sampleSchema)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.IsValid {
				t.Fatalf("expected validation failure for %s", tt.name)
			}
			found := false
			for _, e := range res.Errors {
				if e.Field == tt.expectedField {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error on field '%s', got errors: %+v", tt.expectedField, res.Errors)
			}
		})
	}
}

func TestValidatorCaching(t *testing.T) {
	v := New()
	validEvent := []byte(`{
		"specversion": "1.0",
		"id": "evt-1",
		"source": "/test",
		"type": "test.event",
		"data": {
			"order_id": "ord_123",
			"amount": 25.0,
			"currency": "USD",
			"customer": {"email": "c@example.com"}
		}
	}`)

	// First call compiles and caches schema
	res1, err := v.ValidateCloudEvent(validEvent, sampleSchema)
	if err != nil || !res1.IsValid {
		t.Fatalf("expected call 1 to succeed: %v, %+v", err, res1)
	}

	// Second call hits cache
	res2, err := v.ValidateCloudEvent(validEvent, sampleSchema)
	if err != nil || !res2.IsValid {
		t.Fatalf("expected call 2 to succeed from cache: %v, %+v", err, res2)
	}

	v.cacheMu.RLock()
	cacheLen := len(v.cache)
	v.cacheMu.RUnlock()

	if cacheLen != 1 {
		t.Errorf("expected 1 cached schema, got %d", cacheLen)
	}
}

func TestValidateData_Direct(t *testing.T) {
	v := New()
	schema := `{
		"properties": {
			"code": {"type": "string", "pattern": "^[A-Z]{3}$"},
			"score": {"type": "integer", "minimum": 0, "maximum": 100},
			"nullable_field": {"type": "null"}
		},
		"required": ["code", "score"]
	}`

	// Valid data
	validMap := map[string]any{
		"code":           "XYZ",
		"score":          95,
		"nullable_field": nil,
	}
	res, err := v.ValidateData(validMap, schema)
	if err != nil || !res.IsValid {
		t.Fatalf("expected valid data, got error: %v, errors: %+v", err, res.Errors)
	}

	// Invalid pattern and max value
	invalidMap := map[string]any{
		"code":  "lowercase",
		"score": 150,
	}
	res, err = v.ValidateData(invalidMap, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsValid {
		t.Fatalf("expected validation failure")
	}
	if len(res.Errors) < 2 {
		t.Errorf("expected at least 2 errors, got %d", len(res.Errors))
	}
}
