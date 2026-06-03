package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadJSONC reads a JSON or JSONC file, stripping comments if the extension is .jsonc.
func ReadJSONC(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	clean := string(data)
	if strings.HasSuffix(path, ".jsonc") {
		clean = stripJSONCComments(clean)
	}

	var root map[string]any
	if err := json.Unmarshal([]byte(clean), &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return root, nil
}

// WriteJSONC writes a map as JSON (always without comments) to the given path.
func WriteJSONC(path string, root map[string]any) error {
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// FindJSONCPath returns {name}.jsonc if it exists, otherwise {name}.json.
func FindJSONCPath(dir, name string) string {
	jsonc := filepath.Join(dir, name+".jsonc")
	if _, err := os.Stat(jsonc); err == nil {
		return jsonc
	}
	return filepath.Join(dir, name+".json")
}

func stripJSONCComments(input string) string {
	var out strings.Builder
	inString := false
	escape := false
	i := 0
	for i < len(input) {
		c := input[i]

		if inString {
			if escape {
				escape = false
			} else if c == '\\' {
				escape = true
			} else if c == '"' {
				inString = false
			}
			out.WriteByte(c)
			i++
			continue
		}

		if c == '"' {
			inString = true
			out.WriteByte(c)
			i++
			continue
		}

		// Check for comments
		if c == '/' && i+1 < len(input) {
			next := input[i+1]
			if next == '/' {
				// Line comment: skip to end of line
				for i < len(input) && input[i] != '\n' {
					i++
				}
				continue
			} else if next == '*' {
				// Block comment: skip to */
				i += 2
				for i < len(input) {
					if input[i] == '*' && i+1 < len(input) && input[i+1] == '/' {
						i += 2
						break
					}
					i++
				}
				continue
			}
		}

		out.WriteByte(c)
		i++
	}

	return out.String()
}
