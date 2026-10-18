package codegen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

const sampleSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "OrderCompletedPayload",
  "type": "object",
  "properties": {
    "order_id": { "type": "string", "description": "Unique order identifier" },
    "customer_email": { "type": "string", "description": "Customer contact email" },
    "amount": { "type": "number", "description": "Total purchase amount" },
    "country": { "type": "string", "description": "ISO 2-letter country code" },
    "is_priority": { "type": "boolean", "description": "High priority order flag" },
    "item_ids": { "type": "array", "items": { "type": "string" }, "description": "List of ordered items" }
  },
  "required": ["order_id", "customer_email", "amount", "country"]
}`

func getSampleContract() EventContract {
	schemaDef, err := ParseJSONSchema(sampleSchema)
	if err != nil {
		panic(err)
	}
	return EventContract{
		ID:          "ev_orders_completed",
		Name:        "OrderCompleted",
		Type:        "com.company.orders.completed.v1",
		SemVer:      "1.0.0",
		Description: "Emitted when a customer checkout is confirmed and paid.",
		Deprecated:  false,
		Schema:      schemaDef,
	}
}

func TestParseJSONSchema(t *testing.T) {
	def, err := ParseJSONSchema(sampleSchema)
	if err != nil {
		t.Fatalf("ParseJSONSchema failed: %v", err)
	}

	if def.Title != "OrderCompletedPayload" {
		t.Errorf("expected title 'OrderCompletedPayload', got '%s'", def.Title)
	}

	if len(def.Properties) != 6 {
		t.Fatalf("expected 6 properties, got %d", len(def.Properties))
	}

	// Verify required flags
	propMap := make(map[string]PropertySchema)
	for _, p := range def.Properties {
		propMap[p.Name] = p
	}

	if !propMap["order_id"].Required {
		t.Errorf("expected order_id to be required")
	}
	if !propMap["country"].Required {
		t.Errorf("expected country to be required")
	}
	if propMap["is_priority"].Required {
		t.Errorf("expected is_priority to be optional")
	}
	if propMap["item_ids"].ItemType != "string" {
		t.Errorf("expected item_ids ItemType 'string', got '%s'", propMap["item_ids"].ItemType)
	}
}

func TestGenerateTypeScript(t *testing.T) {
	contract := getSampleContract()
	tsCode, err := GenerateTypeScript(contract)
	if err != nil {
		t.Fatalf("GenerateTypeScript failed: %v", err)
	}

	// Assert essential constructs
	checks := []string{
		"export interface OrderCompletedPayload {",
		"order_id: string;",
		"amount: number;",
		"country: string;",
		"is_priority?: boolean;",
		"item_ids?: string[];",
		"export interface OrderCompletedCloudEvent {",
		"specversion: \"1.0\";",
		`type: "com.company.orders.completed.v1";`,
		"data: OrderCompletedPayload;",
		"export function validateOrderCompletedPayload(data: unknown): data is OrderCompletedPayload",
		"export function validateOrderCompletedCloudEvent(event: unknown): event is OrderCompletedCloudEvent",
		"export function createOrderCompletedCloudEvent(",
	}

	for _, check := range checks {
		if !strings.Contains(tsCode, check) {
			t.Errorf("TypeScript output missing expected string: %s\nGenerated code:\n%s", check, tsCode)
		}
	}
}

func TestGeneratePython(t *testing.T) {
	contract := getSampleContract()
	pyCode, err := GeneratePython(contract)
	if err != nil {
		t.Fatalf("GeneratePython failed: %v", err)
	}

	checks := []string{
		"from pydantic import BaseModel, Field",
		"class OrderCompletedPayload(BaseModel):",
		"order_id: str = Field(description=\"Unique order identifier\")",
		"amount: float = Field(description=\"Total purchase amount\")",
		"is_priority: Optional[bool] = Field(default=None",
		"item_ids: Optional[List[str]] = Field(default=None",
		"class OrderCompletedCloudEvent(BaseModel):",
		`type: str = Field(default="com.company.orders.completed.v1")`,
		"data: OrderCompletedPayload",
		"def create_order_completed_cloud_event(",
		"-> OrderCompletedCloudEvent:",
	}

	for _, check := range checks {
		if !strings.Contains(pyCode, check) {
			t.Errorf("Python output missing expected string: %s\nGenerated code:\n%s", check, pyCode)
		}
	}
}

func TestGenerateGo(t *testing.T) {
	contract := getSampleContract()
	goCode, err := GenerateGo(contract, "events")
	if err != nil {
		t.Fatalf("GenerateGo failed: %v", err)
	}

	checks := []string{
		"package events",
		"type OrderCompletedPayload struct {",
		"OrderID",
		`json:"order_id"`,
		"Amount",
		`json:"amount"`,
		"IsPriority",
		`json:"is_priority,omitempty"`,
		"ItemIds",
		`json:"item_ids,omitempty"`,
		"type OrderCompletedCloudEvent struct {",
		"SpecVersion",
		`json:"specversion"`,
		"ID",
		`json:"id"`,
		"Type",
		`json:"type"`,
		"Data",
		`json:"data"`,
		"func CreateOrderCompletedCloudEvent(data OrderCompletedPayload, source string",
	}

	for _, check := range checks {
		if !strings.Contains(goCode, check) {
			t.Errorf("Go output missing expected string: %s\nGenerated code:\n%s", check, goCode)
		}
	}

	// Validate syntax with go/parser
	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "order_completed.go", goCode, parser.AllErrors)
	if parseErr != nil {
		t.Fatalf("Generated Go code has syntax errors: %v\nCode:\n%s", parseErr, goCode)
	}
}

func TestDeprecatedContractRFC8594Warning(t *testing.T) {
	contract := getSampleContract()
	contract.Deprecated = true
	contract.SunsetDate = "2027-01-01T00:00:00Z"

	tsCode, err := GenerateTypeScript(contract)
	if err != nil {
		t.Fatalf("GenerateTypeScript failed: %v", err)
	}
	if !strings.Contains(tsCode, "@deprecated RFC 8594") || !strings.Contains(tsCode, "2027-01-01T00:00:00Z") {
		t.Errorf("TypeScript output missing RFC 8594 deprecation tag")
	}

	pyCode, err := GeneratePython(contract)
	if err != nil {
		t.Fatalf("GeneratePython failed: %v", err)
	}
	if !strings.Contains(pyCode, "per RFC 8594") || !strings.Contains(pyCode, "2027-01-01T00:00:00Z") {
		t.Errorf("Python output missing RFC 8594 deprecation warning")
	}

	goCode, err := GenerateGo(contract, "events")
	if err != nil {
		t.Fatalf("GenerateGo failed: %v", err)
	}
	if !strings.Contains(goCode, "RFC 8594 sunset warning") || !strings.Contains(goCode, "2027-01-01T00:00:00Z") {
		t.Errorf("Go output missing RFC 8594 deprecation warning")
	}
}
