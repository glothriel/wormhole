package cmd

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSanitizeStringFlag(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "string with spaces",
			input:    "  test  ",
			expected: "--test--",
		},
		{
			name:     "string with special characters",
			input:    "test!@#$%^&*()_+",
			expected: "test------------",
		},
		{
			name:     "string with numbers",
			input:    "123456",
			expected: "123456",
		},
		{
			name:     "string with special characters and spaces",
			input:    "  test!@#$%^&*()_+  ",
			expected: "--test--------------",
		},
		{
			name:     "string with special characters, spaces, and numbers",
			input:    "  test!@#$%^&*()_+ 123456 ",
			expected: "--test-------------123456-",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, sanitizeStringFlag(tt.input))
		})
	}
}
