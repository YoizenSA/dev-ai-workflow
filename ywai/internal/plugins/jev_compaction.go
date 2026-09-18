package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultCompactionThreshold mirrors the jev-compaction plugin default.
const DefaultCompactionThreshold = 200000

// CompactionThreshold reads compactionThresholdTokens from the ywai-managed
// jev-gate.json; set is false when the file does not define it.
func CompactionThreshold() (tokens int, set bool) {
	fields := readJevFile()
	if v, ok := fields["compactionThresholdTokens"].(float64); ok && v > 0 {
		return int(v), true
	}
	return DefaultCompactionThreshold, false
}

// SetCompactionThreshold writes compactionThresholdTokens into the same file
// as the Jev key, keeping every other field (the key included) and mode 0600.
func SetCompactionThreshold(tokens int) error {
	if tokens <= 0 {
		return fmt.Errorf("threshold must be a positive number of tokens, got %d", tokens)
	}
	fields := readJevFile()
	fields["compactionThresholdTokens"] = tokens
	path := JevKeyPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	body, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return err
	}
	// 0600: the file may hold the API key.
	if err := os.WriteFile(path, append(body, '\n'), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func readJevFile() map[string]any {
	fields := map[string]any{}
	if body, err := os.ReadFile(JevKeyPath()); err == nil {
		_ = json.Unmarshal(body, &fields)
	}
	if fields == nil {
		fields = map[string]any{}
	}
	return fields
}
