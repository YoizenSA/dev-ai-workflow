package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// DelegationsFile is the default name of the delegations source-of-truth file,
// expected next to the agent profiles (ywai/agents/delegations.json).
const DelegationsFile = "delegations.json"

// delegationsDoc is the on-disk shape of delegations.json.
type delegationsDoc struct {
	Delegations map[string]map[string]string `json:"delegations"`
}

// LoadDelegations reads the delegations.json next to the agent profiles. The
// map is agent -> { subagent glob: "allow"|"ask"|"deny" }. Returns an empty
// map (not an error) when the file is absent so installs that pre-date this
// feature keep working.
func LoadDelegations(sourceDir string) (map[string]map[string]string, error) {
	path := filepath.Join(sourceDir, DelegationsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]map[string]string{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc delegationsDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if doc.Delegations == nil {
		return map[string]map[string]string{}, nil
	}
	return doc.Delegations, nil
}

// ApplyDelegations writes the per-agent permission.task map from delegations
// into opencode.json (creating the agent entry + permission block as needed).
// It is idempotent: re-running with the same delegations leaves the file
// unchanged. Existing scalar permissions on each agent are preserved; only the
// "task" key of the permission block is replaced (or removed when the agent has
// no delegation rules).
//
// configPath is the opencode.json/.jsonc path. delegations comes from
// LoadDelegations.
func ApplyDelegations(configPath string, delegations map[string]map[string]string) error {
	if len(delegations) == 0 {
		return nil
	}

	root := map[string]any{}
	if _, err := os.Stat(configPath); err == nil {
		r, err := config.ReadJSONC(configPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", configPath, err)
		}
		root = r
	} else {
		if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
			return fmt.Errorf("create config dir: %w", err)
		}
	}

	agentsRaw, ok := root["agent"]
	if !ok {
		agentsRaw = map[string]any{}
		root["agent"] = agentsRaw
	}
	agents, ok := agentsRaw.(map[string]any)
	if !ok {
		agents = map[string]any{}
		root["agent"] = agents
	}

	applied := 0
	for name, taskMap := range delegations {
		entry, ok := agents[name].(map[string]any)
		if !ok {
			entry = map[string]any{}
			agents[name] = entry
		}

		permRaw, ok := entry["permission"]
		var perm map[string]any
		if ok {
			perm, _ = permRaw.(map[string]any)
		}
		if perm == nil {
			perm = map[string]any{}
		}

		if len(taskMap) == 0 {
			delete(perm, "task")
		} else {
			// taskMap is map[string]string; copy into map[string]any so the
			// JSON marshaler emits a plain object (not a nested struct).
			obj := make(map[string]any, len(taskMap))
			for k, v := range taskMap {
				obj[k] = v
			}
			perm["task"] = obj
		}
		entry["permission"] = perm
		applied++
	}

	if applied == 0 {
		return nil
	}

	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	fmt.Printf("  Applied delegation graph to %d agents\n", applied)
	return nil
}
