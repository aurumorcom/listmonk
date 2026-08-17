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
