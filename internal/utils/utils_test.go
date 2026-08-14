package utils

import (
	"testing"
)

func TestSanitizePhone(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		expectErr bool
	}{
		{
			name:      "empty string is valid optional phone",
			input:     "",
			expected:  "",
			expectErr: false,
		},
		{
			name:      "whitespace only is valid optional phone",
			input:     "   \t\n  ",
			expected:  "",
			expectErr: false,
		},
		{
			name:      "valid US standard E.164",
			input:     "+14155552671",
			expected:  "+14155552671",
			expectErr: false,
		},
		{
			name:      "valid US formatted",
			input:     "+1 (415) 555-2671",
			expected:  "+14155552671",
			expectErr: false,
		},
		{
			name:      "valid US without leading plus",
			input:     "14155552671",
			expected:  "+14155552671",
			expectErr: false,
		},
		{
			name:      "valid UK standard E.164",
			input:     "+442079460958",
			expected:  "+442079460958",
			expectErr: false,
		},
		{
			name:      "valid UK formatted",
			input:     "+44 20 7946 0958",
			expected:  "+442079460958",
			expectErr: false,
		},
		{
			name:      "valid India standard E.164",
			input:     "+919472380340",
			expected:  "+919472380340",
			expectErr: false,
		},
		{
			name:      "valid India formatted with spaces",
			input:     "+91 94723 80340",
			expected:  "+919472380340",
			expectErr: false,
		},
		{
			name:      "valid Germany international",
			input:     "+49 30 123456",
			expected:  "+4930123456",
			expectErr: false,
		},
		{
			name:      "valid Brazil international with 00 prefix",
			input:     "005511912345678",
			expected:  "+5511912345678",
			expectErr: false,
		},
		{
			name:      "invalid local number missing country code",
			input:     "0712345678",
			expected:  "",
			expectErr: true,
		},
		{
			name:      "invalid incomplete number",
			input:     "+1 555",
			expected:  "",
			expectErr: true,
		},
		{
			name:      "invalid fake country code",
			input:     "+999 12345678",
			expected:  "",
			expectErr: true,
		},
		{
			name:      "invalid alphanumeric string",
			input:     "+1 800 FLOWERS",
			expected:  "",
			expectErr: true,
		},
		{
			name:      "invalid random text",
			input:     "random-phone-number",
			expected:  "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SanitizePhone(tt.input)
			if (err != nil) != tt.expectErr {
				t.Fatalf("SanitizePhone(%q) error = %v, expectErr %v", tt.input, err, tt.expectErr)
			}
			if got != tt.expected {
				t.Errorf("SanitizePhone(%q) = %q, want %q", tt.input, got, tt.expected)
			}
			if ValidatePhone(tt.input) == tt.expectErr {
				t.Errorf("ValidatePhone(%q) = %v, want %v", tt.input, !tt.expectErr, !tt.expectErr)
			}
		})
	}
}
