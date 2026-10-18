package compatibility

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func generateSchemaWithProps(numProps int, extraOptionalProps int) string {
	props := make(map[string]map[string]string)
	required := make([]string, 0, numProps/2)

	for i := 1; i <= numProps; i++ {
		key := fmt.Sprintf("property_%04d", i)
		props[key] = map[string]string{
			"type":        "string",
			"description": fmt.Sprintf("Field description for property %d", i),
		}
		if i%2 == 0 {
			required = append(required, key)
		}
	}

	for j := 1; j <= extraOptionalProps; j++ {
		key := fmt.Sprintf("extra_optional_%04d", j)
		props[key] = map[string]string{
			"type": "string",
		}
	}

	doc := map[string]any{
		"title":      "BenchmarkingSchema",
		"type":       "object",
		"properties": props,
		"required":   required,
	}

	b, _ := json.Marshal(doc)
	return string(b)
}

func TestCompatibilityLatencyBudget_500Properties(t *testing.T) {
	oldSchema := generateSchemaWithProps(500, 0)
	newSchema := generateSchemaWithProps(500, 20) // Add 20 optional properties (compatible minor change)

	// Allow reasonable overhead for virtualized CI environments and -race detector instrumentation.
	const budget = 25 * time.Millisecond

	// Warm up
	_, err := CheckCompatibility(oldSchema, newSchema)
	if err != nil {
		t.Fatalf("unexpected error during warm up: %v", err)
	}

	iterations := 5
	var minDuration time.Duration = time.Hour

	for i := 0; i < iterations; i++ {
		start := time.Now()
		res, err := CheckCompatibility(oldSchema, newSchema)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("unexpected check error: %v", err)
		}
		if !res.IsCompatible {
			t.Fatalf("expected compatible change, got breaking: %+v", res.Reasons)
		}
		if res.ChangeType != ChangeMinor {
			t.Fatalf("expected ChangeMinor, got %s", res.ChangeType)
		}

		if elapsed < minDuration {
			minDuration = elapsed
		}
	}

	t.Logf("Best CheckCompatibility duration for 500 properties: %v (Latency Budget: %v)", minDuration, budget)

	if minDuration > budget {
		t.Errorf("latency budget exceeded! Best %v > Budget %v", minDuration, budget)
	}
}

func BenchmarkCheckCompatibility_500Properties(b *testing.B) {
	oldSchema := generateSchemaWithProps(500, 0)
	newSchema := generateSchemaWithProps(500, 20)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		res, err := CheckCompatibility(oldSchema, newSchema)
		if err != nil || !res.IsCompatible {
			b.Fatalf("failed check: %v", err)
		}
	}
}
