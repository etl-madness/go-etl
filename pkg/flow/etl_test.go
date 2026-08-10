package flow

import (
	"testing"
)

func TestFormatPlaceholder(t *testing.T) {
	tests := []struct {
		driver   string
		paramIdx int
		expected string
	}{
		{"postgres", 1, "$1"},
		{"postgresql", 5, "$5"},
		{"mysql", 2, "?"},
		{"sqlite3", 3, "?"},
		{"oracle", 4, ":4"},
		{"mssql", 2, "@p2"},
		{"sqlserver", 10, "@p10"},
		{"unknown", 1, "@p1"},
	}

	for _, tt := range tests {
		t.Run(tt.driver, func(t *testing.T) {
			got := formatPlaceholder(tt.driver, tt.paramIdx)
			if got != tt.expected {
				t.Errorf("formatPlaceholder(%q, %d) = %q; want %q", tt.driver, tt.paramIdx, got, tt.expected)
			}
		})
	}
}
