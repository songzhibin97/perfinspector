package locator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestContainsKeyword tests keyword matching
func TestContainsKeyword(t *testing.T) {
	tests := []struct {
		funcName string
		keyword  string
		expected bool
	}{
		{"cacheManager", "cache", true},
		{"CacheManager", "cache", true},
		{"myCache", "Cache", true},
		{"something", "cache", false},
		{"", "cache", false},
		{"cache", "", false},
	}

	for _, tt := range tests {
		result := containsKeyword(tt.funcName, tt.keyword)
		assert.Equal(t, tt.expected, result, "containsKeyword(%q, %q)", tt.funcName, tt.keyword)
	}
}

// TestContainsSubstring tests substring matching
func TestContainsSubstring(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"HelloWorld", "world", true},
		{"HelloWorld", "WORLD", true},
		{"HelloWorld", "xyz", false},
		{"", "test", false},
		{"test", "", true},
	}

	for _, tt := range tests {
		result := containsSubstring(tt.s, tt.substr)
		assert.Equal(t, tt.expected, result, "containsSubstring(%q, %q)", tt.s, tt.substr)
	}
}

// TestToLower tests lowercase conversion
func TestToLower(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"HELLO", "hello"},
		{"Hello", "hello"},
		{"hello", "hello"},
		{"Hello123", "hello123"},
		{"", ""},
	}

	for _, tt := range tests {
		result := toLower(tt.input)
		assert.Equal(t, tt.expected, result, "toLower(%q)", tt.input)
	}
}

// TestIndexOf tests substring index finding
func TestIndexOf(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected int
	}{
		{"hello world", "world", 6},
		{"hello world", "hello", 0},
		{"hello world", "xyz", -1},
		{"hello", "", 0},
		{"", "test", -1},
	}

	for _, tt := range tests {
		result := indexOf(tt.s, tt.substr)
		assert.Equal(t, tt.expected, result, "indexOf(%q, %q)", tt.s, tt.substr)
	}
}
