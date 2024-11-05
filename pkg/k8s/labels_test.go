package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCSVToMap(t *testing.T) {
	cases := []struct {
		name     string
		csv      string
		expected map[string]string
	}{
		{
			name:     "empty",
			csv:      "",
			expected: map[string]string{},
		},
		{
			name:     "single",
			csv:      "key=value",
			expected: map[string]string{"key": "value"},
		},
		{
			name:     "multiple",
			csv:      "key1=value1,key2=value2",
			expected: map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name:     "equal sign",
			csv:      "key1=value1,key2=value2=with=equal",
			expected: map[string]string{"key1": "value1", "key2": "value2=with=equal"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual := CSVToMap(tc.csv)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
