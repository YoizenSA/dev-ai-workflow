package review

import (
	"testing"
)

func TestParseResult(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected ReviewResult
	}{
		{
			name:     "PASSED status",
			output:   "Some text\nSTATUS: PASSED\nMore text",
			expected: ResultPassed,
		},
		{
			name:     "FAILED status",
			output:   "Some text\nSTATUS: FAILED\nMore text",
			expected: ResultFailed,
		},
		{
			name:     "PASSED with markdown",
			output:   "Some text\n**STATUS: PASSED**\nMore text",
			expected: ResultPassed,
		},
		{
			name:     "FAILED with markdown",
			output:   "Some text\n**STATUS: FAILED**\nMore text",
			expected: ResultFailed,
		},
		{
			name:     "Unknown status",
			output:   "Some text\nNo status here\nMore text",
			expected: ResultUnknown,
		},
		{
			name: "PASSED in first 15 lines",
			output: "line1\nline2\nline3\nline4\nline5\nSTATUS: PASSED\n" +
				"line7\nline8\nline9\nline10\nline11\nline12\nline13\nline14\nline15\nline16",
			expected: ResultPassed,
		},
		{
			name: "Unknown after 15 lines",
			output: "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\n" +
				"line10\nline11\nline12\nline13\nline14\nline15\nSTATUS: PASSED",
			expected: ResultUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseResult(tt.output)
			if result != tt.expected {
				t.Errorf("ParseResult() = %v, want %v", result, tt.expected)
			}
		})
	}
}
