//go:build unit || !integration

package models

import (
	"testing"
)

func TestSubstituteTplShorthand_Exhaustive(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Standard TrackLink substitution",
			input:    `{{ TrackLink "https://aurumor.com" }}`,
			expected: `{{ TrackLink "https://aurumor.com" . }}`,
		},
		{
			name:     "WYSIWYG @TrackLink URL shorthand substitution",
			input:    `<a href="https://aurumor.com/link@TrackLink">Click here</a>`,
			expected: `<a href="{{ TrackLink "https://aurumor.com/link" . }}">Click here</a>`,
		},
		{
			name:     "TrackView simple tag substitution",
			input:    `{{ TrackView }}`,
			expected: `{{ TrackView . }}`,
		},
		{
			name:     "UnsubscribeURL tag substitution",
			input:    `{{ UnsubscribeURL }}`,
			expected: `{{ UnsubscribeURL . }}`,
		},
		{
			name:     "ManageURL tag substitution",
			input:    `{{ ManageURL }}`,
			expected: `{{ ManageURL . }}`,
		},
		{
			name:     "OptinURL tag substitution",
			input:    `{{ OptinURL }}`,
			expected: `{{ OptinURL . }}`,
		},
		{
			name:     "MessageURL tag substitution",
			input:    `{{ MessageURL }}`,
			expected: `{{ MessageURL . }}`,
		},
		{
			name:     "Plain text without template shorthand",
			input:    `Hello world, no template tags here.`,
			expected: `Hello world, no template tags here.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SubstituteTplShorthand(tt.input)
			if result != tt.expected {
				t.Errorf("SubstituteTplShorthand() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestJSON_Scan_NilTarget(t *testing.T) {
	var target JSON
	if target != nil {
		t.Fatalf("expected initial target map to be nil")
	}

	payload := []byte(`{"name":"Alex Lead","company":"Acme Corp","active":true}`)
	if err := target.Scan(payload); err != nil {
		t.Fatalf("expected nil-target Scan to succeed, got: %v", err)
	}

	if target == nil {
		t.Fatalf("expected target to be populated after Scan, got nil")
	}
	if target["name"] != "Alex Lead" || target["company"] != "Acme Corp" || target["active"] != true {
		t.Fatalf("unexpected unmarshaled map contents: %+v", target)
	}
}

func TestJSON_Scan_StringInput(t *testing.T) {
	var target JSON
	strPayload := `{"days":["mon","tue","wed"],"start_time":"09:00","end_time":"17:00"}`

	if err := target.Scan(strPayload); err != nil {
		t.Fatalf("expected string input Scan to succeed, got: %v", err)
	}

	if target == nil {
		t.Fatalf("expected non-nil map after string Scan")
	}
	if target["start_time"] != "09:00" || target["end_time"] != "17:00" {
		t.Fatalf("unexpected values: %+v", target)
	}
}

func TestJSON_Scan_NilInput(t *testing.T) {
	var target JSON
	if err := target.Scan(nil); err != nil {
		t.Fatalf("expected nil input Scan to succeed, got: %v", err)
	}

	if target == nil {
		t.Fatalf("expected initialized empty map after nil Scan")
	}
	if len(target) != 0 {
		t.Fatalf("expected empty map, got len=%d", len(target))
	}
}

func TestJSON_Scan_InvalidTypesAndSyntax(t *testing.T) {
	var target JSON

	// Unsupported type (int)
	if err := target.Scan(12345); err == nil {
		t.Fatalf("expected int scan to return error, got nil")
	}

	// Invalid JSON syntax
	if err := target.Scan([]byte(`{invalid-json`)); err == nil {
		t.Fatalf("expected malformed JSON scan to return error, got nil")
	}
}

func TestJSON_Value(t *testing.T) {
	var nilTarget JSON
	val, err := nilTarget.Value()
	if err != nil {
		t.Fatalf("expected nil target Value() to succeed, got: %v", err)
	}
	if string(val.([]byte)) != "{}" {
		t.Fatalf("expected empty object '{}' for nil map Value(), got: %s", val)
	}

	populated := JSON{"key": "value"}
	val2, err := populated.Value()
	if err != nil {
		t.Fatalf("expected populated Value() to succeed: %v", err)
	}
	if string(val2.([]byte)) != `{"key":"value"}` {
		t.Fatalf("expected marshaled json, got: %s", val2)
	}
}

func TestStringIntMap_Scan(t *testing.T) {
	var target StringIntMap
	if target != nil {
		t.Fatalf("expected initial target map to be nil")
	}

	// Byte slice scan
	if err := target.Scan([]byte(`{"a":1,"b":2}`)); err != nil {
		t.Fatalf("expected byte slice Scan to succeed: %v", err)
	}
	if target["a"] != 1 || target["b"] != 2 {
		t.Fatalf("unexpected StringIntMap values: %+v", target)
	}

	// String scan
	var target2 StringIntMap
	if err := target2.Scan(`{"c":3}`); err != nil {
		t.Fatalf("expected string Scan to succeed: %v", err)
	}
	if target2["c"] != 3 {
		t.Fatalf("unexpected value: %+v", target2)
	}

	// Nil scan
	var target3 StringIntMap
	if err := target3.Scan(nil); err != nil {
		t.Fatalf("expected nil Scan to succeed: %v", err)
	}
	if target3 == nil || len(target3) != 0 {
		t.Fatalf("expected empty StringIntMap, got: %+v", target3)
	}
}
