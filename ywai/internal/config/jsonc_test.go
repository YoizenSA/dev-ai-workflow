package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStripJSONCComments(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "line comment",
			input: `{"a": 1} // comment`,
			want:  `{"a": 1} `,
		},
		{
			name:  "block comment",
			input: `/* header */ {"a": 1}`,
			want:  ` {"a": 1}`,
		},
		{
			name:  "comment inside string preserved",
			input: `{"url": "http://example.com"}`,
			want:  `{"url": "http://example.com"}`,
		},
		{
			name:  "mixed comments",
			input: "// top\n{\"a\": 1}\n/* tail */",
			want:  "\n{\"a\": 1}\n",
		},
		{
			name:  "no comments",
			input: `{"a": 1}`,
			want:  `{"a": 1}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripJSONCComments(tt.input)
			if got != tt.want {
				t.Errorf("stripJSONCComments() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadJSONC_WithComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonc")

	content := `// config for opencode
{
  "agent": {
    /* primary agent */
    "ask": {
      "mode": "all"
    }
  }
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	root, err := ReadJSONC(path)
	if err != nil {
		t.Fatalf("ReadJSONC error = %v", err)
	}

	agent := root["agent"].(map[string]any)
	ask := agent["ask"].(map[string]any)
	if ask["mode"] != "all" {
		t.Errorf("mode = %v", ask["mode"])
	}
}

func TestWriteJSONC_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	root := map[string]any{"agent": map[string]any{"ask": map[string]any{"mode": "all"}}}
	if err := WriteJSONC(path, root); err != nil {
		t.Fatalf("WriteJSONC error = %v", err)
	}

	read, err := ReadJSONC(path)
	if err != nil {
		t.Fatalf("ReadJSONC error = %v", err)
	}

	agent := read["agent"].(map[string]any)
	ask := agent["ask"].(map[string]any)
	if ask["mode"] != "all" {
		t.Errorf("mode = %v", ask["mode"])
	}
}
