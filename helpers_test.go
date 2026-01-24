package main

import "testing"

func TestUnescapeICalText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "lowercase \\n escape sequence",
			input:    "Line 1\\nLine 2",
			expected: "Line 1\nLine 2",
		},
		{
			name:     "uppercase \\N escape sequence",
			input:    "Line 1\\NLine 2",
			expected: "Line 1\nLine 2",
		},
		{
			name:     "multiple escape sequences",
			input:    "Line 1\\nLine 2\\NLine 3",
			expected: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "no escape sequences",
			input:    "Simple text",
			expected: "Simple text",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "escape sequence at start",
			input:    "\\nLine 1",
			expected: "\nLine 1",
		},
		{
			name:     "escape sequence at end",
			input:    "Line 1\\n",
			expected: "Line 1\n",
		},
		{
			name:     "mixed case escape sequences",
			input:    "Part 1\\nPart 2\\NPart 3",
			expected: "Part 1\nPart 2\nPart 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := unescapeICalText(tt.input)
			if result != tt.expected {
				t.Errorf("unescapeICalText(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
