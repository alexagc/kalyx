package validator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/alexagc/kalyx/internal/httputil"
)

// ValidationError describes a single schema or envelope violation.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ToInvalidParam converts a ValidationError into an RFC 7807 InvalidParam.
func (v ValidationError) ToInvalidParam() httputil.InvalidParam {
	return httputil.InvalidParam{
		Name:   v.Field,
		Reason: v.Message,
	}
}

// ValidationResult summarizes the outcome of validating a CloudEvent.
type ValidationResult struct {
	IsValid bool              `json:"is_valid"`
	Errors  []ValidationError `json:"errors,omitempty"`
}

// ToInvalidParams converts all validation errors to RFC 7807 invalid params.
func (r *ValidationResult) ToInvalidParams() []httputil.InvalidParam {
	if len(r.Errors) == 0 {
		return nil
	}
	params := make([]httputil.InvalidParam, len(r.Errors))
	for i, err := range r.Errors {
		params[i] = err.ToInvalidParam()
	}
	return params
}

// Validator defines the interface for CloudEvents schema validation.
type Validator interface {
	ValidateCloudEvent(eventJSON []byte, schemaJSON string) (*ValidationResult, error)
	ValidateData(data any, schemaJSON string) (*ValidationResult, error)
	ValidateSchema(schemaJSON string) error
}

// CompiledProperty represents a compiled property in JSON Schema Draft 2020-12.
type CompiledProperty struct {
	Type        string
	Format      string
	Minimum     *float64
	Maximum     *float64
	MinLength   *int
	MaxLength   *int
	Pattern     *regexp.Regexp
	Enum        []any
	Required    []string
	Properties  map[string]*CompiledProperty
	Items       *CompiledProperty
}

// CompiledSchema represents a parsed and cached JSON Schema.
type CompiledSchema struct {
	Title      string
	Properties map[string]*CompiledProperty
	Required   []string
}

// DefaultValidator implements Validator with thread-safe schema caching.
type DefaultValidator struct {
	cacheMu sync.RWMutex
	cache   map[string]*CompiledSchema
}

// New creates a new DefaultValidator instance.
func New() *DefaultValidator {
	return &DefaultValidator{
		cache: make(map[string]*CompiledSchema),
	}
}

// CloudEventEnvelope captures the mandatory and optional top-level CloudEvents attributes.
type CloudEventEnvelope struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Source          string          `json:"source"`
	Type            string          `json:"type"`
	DataContentType string          `json:"datacontenttype,omitempty"`
	DataSchema      string          `json:"dataschema,omitempty"`
	Subject         string          `json:"subject,omitempty"`
	Time            string          `json:"time,omitempty"`
	Data            json.RawMessage `json:"data"`
}

// ValidateCloudEvent validates both the CloudEvents 1.0.2 envelope and the data payload against schemaJSON.
func (v *DefaultValidator) ValidateCloudEvent(eventJSON []byte, schemaJSON string) (*ValidationResult, error) {
	var env CloudEventEnvelope
	if err := json.Unmarshal(eventJSON, &env); err != nil {
		return &ValidationResult{
			IsValid: false,
			Errors: []ValidationError{
				{Field: "envelope", Message: fmt.Sprintf("malformed json: %v", err)},
			},
		}, nil
	}

	var errors []ValidationError

	// 1. Validate CloudEvents 1.0.2 Envelope
	if env.SpecVersion == "" {
		errors = append(errors, ValidationError{Field: "specversion", Message: "required attribute 'specversion' is missing"})
	} else if env.SpecVersion != "1.0" {
		errors = append(errors, ValidationError{Field: "specversion", Message: fmt.Sprintf("specversion must be '1.0', got '%s'", env.SpecVersion)})
	}

	if env.ID == "" {
		errors = append(errors, ValidationError{Field: "id", Message: "required attribute 'id' is missing"})
	}
	if env.Source == "" {
		errors = append(errors, ValidationError{Field: "source", Message: "required attribute 'source' is missing"})
	}
	if env.Type == "" {
		errors = append(errors, ValidationError{Field: "type", Message: "required attribute 'type' is missing"})
	}

	if env.Time != "" {
		if _, err := time.Parse(time.RFC3339, env.Time); err != nil {
			errors = append(errors, ValidationError{Field: "time", Message: "attribute 'time' must be a valid RFC3339 timestamp"})
		}
	}

	// 2. Validate Data Payload against JSON Schema
	if schemaJSON != "" {
		compiled, err := v.getOrCompileSchema(schemaJSON)
		if err != nil {
			return nil, fmt.Errorf("invalid json schema: %w", err)
		}

		if len(env.Data) == 0 || string(env.Data) == "null" {
			if len(compiled.Required) > 0 || len(compiled.Properties) > 0 {
				errors = append(errors, ValidationError{Field: "data", Message: "payload data is missing or empty"})
			}
		} else {
			var dataPayload any
			if err := json.Unmarshal(env.Data, &dataPayload); err != nil {
				errors = append(errors, ValidationError{Field: "data", Message: fmt.Sprintf("data payload is not valid JSON: %v", err)})
			} else {
				dataErrors := v.validatePayloadAgainstSchema("data", dataPayload, compiled)
				errors = append(errors, dataErrors...)
			}
		}
	}

	return &ValidationResult{
		IsValid: len(errors) == 0,
		Errors:  errors,
	}, nil
}

// ValidateData validates a raw data object directly against a JSON Schema.
func (v *DefaultValidator) ValidateData(data any, schemaJSON string) (*ValidationResult, error) {
	compiled, err := v.getOrCompileSchema(schemaJSON)
	if err != nil {
		return nil, fmt.Errorf("invalid json schema: %w", err)
	}

	errors := v.validatePayloadAgainstSchema("data", data, compiled)
	return &ValidationResult{
		IsValid: len(errors) == 0,
		Errors:  errors,
	}, nil
}

// ValidateSchema verifies that schemaJSON is syntactically valid JSON Schema Draft 2020-12.
func (v *DefaultValidator) ValidateSchema(schemaJSON string) error {
	if strings.TrimSpace(schemaJSON) == "" {
		return errors.New("schema cannot be empty")
	}
	_, err := v.getOrCompileSchema(schemaJSON)
	return err
}

func (v *DefaultValidator) getOrCompileSchema(schemaJSON string) (*CompiledSchema, error) {
	hash := sha256.Sum256([]byte(schemaJSON))
	key := hex.EncodeToString(hash[:])

	v.cacheMu.RLock()
	compiled, found := v.cache[key]
	v.cacheMu.RUnlock()
	if found {
		return compiled, nil
	}

	var rawDoc struct {
		Title      string                    `json:"title"`
		Properties map[string]rawProperty    `json:"properties"`
		Required   []string                  `json:"required"`
	}

	if err := json.Unmarshal([]byte(schemaJSON), &rawDoc); err != nil {
		return nil, err
	}

	cs := &CompiledSchema{
		Title:      rawDoc.Title,
		Properties: make(map[string]*CompiledProperty),
		Required:   rawDoc.Required,
	}

	for k, rp := range rawDoc.Properties {
		cp, err := compileProperty(rp)
		if err != nil {
			return nil, err
		}
		cs.Properties[k] = cp
	}

	v.cacheMu.Lock()
	v.cache[key] = cs
	v.cacheMu.Unlock()

	return cs, nil
}

type rawProperty struct {
	Type        string                 `json:"type"`
	Format      string                 `json:"format,omitempty"`
	Description string                 `json:"description,omitempty"`
	Minimum     *float64               `json:"minimum,omitempty"`
	Maximum     *float64               `json:"maximum,omitempty"`
	MinLength   *int                   `json:"minLength,omitempty"`
	MaxLength   *int                   `json:"maxLength,omitempty"`
	Pattern     string                 `json:"pattern,omitempty"`
	Enum        []any                  `json:"enum,omitempty"`
	Required    []string               `json:"required,omitempty"`
	Properties  map[string]rawProperty `json:"properties,omitempty"`
	Items       *rawProperty           `json:"items,omitempty"`
}

func compileProperty(rp rawProperty) (*CompiledProperty, error) {
	cp := &CompiledProperty{
		Type:      rp.Type,
		Format:    rp.Format,
		Minimum:   rp.Minimum,
		Maximum:   rp.Maximum,
		MinLength: rp.MinLength,
		MaxLength: rp.MaxLength,
		Enum:      rp.Enum,
		Required:  rp.Required,
	}

	if rp.Pattern != "" {
		re, err := regexp.Compile(rp.Pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern regex '%s': %w", rp.Pattern, err)
		}
		cp.Pattern = re
	}

	if len(rp.Properties) > 0 {
		cp.Properties = make(map[string]*CompiledProperty)
		for k, child := range rp.Properties {
			childCompiled, err := compileProperty(child)
			if err != nil {
				return nil, err
			}
			cp.Properties[k] = childCompiled
		}
	}

	if rp.Items != nil {
		itemCompiled, err := compileProperty(*rp.Items)
		if err != nil {
			return nil, err
		}
		cp.Items = itemCompiled
	}

	return cp, nil
}

func (v *DefaultValidator) validatePayloadAgainstSchema(prefix string, data any, cs *CompiledSchema) []ValidationError {
	var errs []ValidationError

	dataMap, ok := data.(map[string]any)
	if !ok {
		return []ValidationError{{Field: prefix, Message: "expected JSON object for data payload"}}
	}

	// Check top-level required properties
	for _, req := range cs.Required {
		if _, exists := dataMap[req]; !exists {
			field := fmt.Sprintf("%s.%s", prefix, req)
			errs = append(errs, ValidationError{Field: field, Message: "field is required"})
		}
	}

	// Validate properties that are present
	for propName, propVal := range dataMap {
		propSchema, defined := cs.Properties[propName]
		if !defined {
			continue
		}
		fieldPath := fmt.Sprintf("%s.%s", prefix, propName)
		propErrs := validatePropertyValue(fieldPath, propVal, propSchema)
		errs = append(errs, propErrs...)
	}

	return errs
}

func validatePropertyValue(field string, val any, prop *CompiledProperty) []ValidationError {
	if val == nil {
		if prop.Type != "null" {
			return []ValidationError{{Field: field, Message: fmt.Sprintf("expected type '%s', got null", prop.Type)}}
		}
		return nil
	}

	var errs []ValidationError

	switch prop.Type {
	case "string":
		s, ok := val.(string)
		if !ok {
			return []ValidationError{{Field: field, Message: "expected string value"}}
		}
		if prop.MinLength != nil && len(s) < *prop.MinLength {
			errs = append(errs, ValidationError{Field: field, Message: fmt.Sprintf("string length must be >= %d", *prop.MinLength)})
		}
		if prop.MaxLength != nil && len(s) > *prop.MaxLength {
			errs = append(errs, ValidationError{Field: field, Message: fmt.Sprintf("string length must be <= %d", *prop.MaxLength)})
		}
		if prop.Pattern != nil && !prop.Pattern.MatchString(s) {
			errs = append(errs, ValidationError{Field: field, Message: fmt.Sprintf("value does not match pattern '%s'", prop.Pattern.String())})
		}
		if prop.Format != "" {
			switch prop.Format {
			case "date-time":
				if _, err := time.Parse(time.RFC3339, s); err != nil {
					errs = append(errs, ValidationError{Field: field, Message: "value must be a valid RFC3339 date-time"})
				}
			case "date":
				if _, err := time.Parse("2006-01-02", s); err != nil {
					errs = append(errs, ValidationError{Field: field, Message: "value must be a valid date (YYYY-MM-DD)"})
				}
			case "email":
				if _, err := mail.ParseAddress(s); err != nil {
					errs = append(errs, ValidationError{Field: field, Message: "value must be a valid email address"})
				}
			}
		}

	case "number":
		num, ok := toFloat64(val)
		if !ok {
			return []ValidationError{{Field: field, Message: "expected number value"}}
		}
		if prop.Minimum != nil && num < *prop.Minimum {
			errs = append(errs, ValidationError{Field: field, Message: fmt.Sprintf("value must be >= %v", *prop.Minimum)})
		}
		if prop.Maximum != nil && num > *prop.Maximum {
			errs = append(errs, ValidationError{Field: field, Message: fmt.Sprintf("value must be <= %v", *prop.Maximum)})
		}

	case "integer":
		num, ok := toFloat64(val)
		if !ok || num != float64(int64(num)) {
			return []ValidationError{{Field: field, Message: "expected integer value"}}
		}
		if prop.Minimum != nil && num < *prop.Minimum {
			errs = append(errs, ValidationError{Field: field, Message: fmt.Sprintf("value must be >= %v", *prop.Minimum)})
		}
		if prop.Maximum != nil && num > *prop.Maximum {
			errs = append(errs, ValidationError{Field: field, Message: fmt.Sprintf("value must be <= %v", *prop.Maximum)})
		}

	case "boolean":
		if _, ok := val.(bool); !ok {
			return []ValidationError{{Field: field, Message: "expected boolean value"}}
		}

	case "array":
		arr, ok := val.([]any)
		if !ok {
			return []ValidationError{{Field: field, Message: "expected array value"}}
		}
		if prop.Items != nil {
			for i, item := range arr {
				itemPath := fmt.Sprintf("%s[%d]", field, i)
				itemErrs := validatePropertyValue(itemPath, item, prop.Items)
				errs = append(errs, itemErrs...)
			}
		}

	case "object":
		obj, ok := val.(map[string]any)
		if !ok {
			return []ValidationError{{Field: field, Message: "expected object value"}}
		}
		for _, req := range prop.Required {
			if _, exists := obj[req]; !exists {
				errs = append(errs, ValidationError{Field: fmt.Sprintf("%s.%s", field, req), Message: "field is required"})
			}
		}
		for subK, subV := range obj {
			if childProp, exists := prop.Properties[subK]; exists {
				childPath := fmt.Sprintf("%s.%s", field, subK)
				childErrs := validatePropertyValue(childPath, subV, childProp)
				errs = append(errs, childErrs...)
			}
		}
	}

	if len(prop.Enum) > 0 {
		matched := false
		for _, e := range prop.Enum {
			if fmt.Sprintf("%v", e) == fmt.Sprintf("%v", val) {
				matched = true
				break
			}
		}
		if !matched {
			errs = append(errs, ValidationError{Field: field, Message: fmt.Sprintf("value must be one of %v", prop.Enum)})
		}
	}

	return errs
}

func toFloat64(val any) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
