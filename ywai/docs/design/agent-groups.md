# Agent Groups — Design Document

**Status**: Proposed  
**Author**: Architect  
**Date**: 2026-06-04

## 1. Overview

Currently, `ywai install` installs ALL agent profiles (orchestrator, ask, dev, qa, architect, reviewer, devops, finder) to every detected AI agent. This design introduces **optional agent groups** so users install only what they need.

## 2. Design Decision

**Chosen approach**: `groups.json` manifest file at `ywai/agents/groups.json`.

**Rationale**:
- **No directory restructure needed** — agents stay in their current `ywai/agents/{name}/` directories.
- **Explicit mapping** — clearer than convention-based grouping (e.g., `agents/core/` subdirs).
- **Keeps AGENT.md clean** — no metadata pollution in agent prompts.
- **Human-editable** — JSON is easy to read and modify.
- **groups.local.json** — gitignored override file for custom/user-specific groups.

**Alternatives considered**:
- Directory convention (`agents/core/`, `agents/social-refactor/`): Requires moving files, breaks existing paths.
- Inline metadata in AGENT.md frontmatter: Pollutes agent prompts with non-prompt metadata.
- Separate config file in ~/.config: Too far from the source of truth (the agents themselves).

## 3. groups.json Schema

**Location**: `ywai/agents/groups.json` (embedded into binary)

```json
{
  "groups": {
    "core": {
      "description": "Core agent profiles — always installed",
      "agents": [
        "orchestrator",
        "ask",
        "dev",
        "qa",
        "architect",
        "reviewer",
        "devops",
        "finder"
      ]
    },
    "social-refactor": {
      "description": "Social refactoring and collaboration agents",
      "agents": []
    }
  }
}
```

**Rules**:
- `groups` is a `map[string]GroupDefinition`
- An agent can belong to only ONE group (first match wins in order of groups map)
- Agent names in `agents[]` must match directory names under `ywai/agents/`
- The `"core"` group is always installed (non-optional)
- `groups.local.json` in the same directory (gitignored) allows user overrides. If present, it's merged with `groups.json` (local wins for same keys).

## 4. Data Structures

```go
// GroupManifest represents the groups.json file
type GroupManifest struct {
    Groups map[string]GroupDefinition `json:"groups"`
}

// GroupDefinition defines a single agent group
type GroupDefinition struct {
    Description string   `json:"description"`
    Agents      []string `json:"agents"`
}

// GroupFilter controls which groups to install
type GroupFilter struct {
    Groups    []string // group names to include (in addition to core)
    AllGroups bool     // install ALL groups (backward compat)
}
```

`AgentProfile` struct remains unchanged.

## 5. Implementation Changes

### 5.1 NEW: `ywai/agents/groups.json`
As defined in Section 3. Embedded into binary alongside agent profiles.

### 5.2 MODIFY: `ywai/internal/agents/agents.go`

**New functions**:

```go
// LoadGroupManifest loads and parses groups.json
func LoadGroupManifest(sourceDir string) (*GroupManifest, error) {
    // Try groups.json first, then groups.local.json and merge
    manifest := &GroupManifest{Groups: make(map[string]GroupDefinition)}
    
    // Load base manifest
    path := filepath.Join(sourceDir, "groups.json")
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("groups.json not found: %w", err)
    }
    var base GroupManifest
    if err := json.Unmarshal(data, &base); err != nil {
        return nil, fmt.Errorf("invalid groups.json: %w", err)
    }
    for k, v := range base.Groups {
        manifest.Groups[k] = v
    }
    
    // Try local override
    localPath := filepath.Join(sourceDir, "groups.local.json")
    if localData, err := os.ReadFile(localPath); err == nil {
        var local GroupManifest
        if err := json.Unmarshal(localData, &local); err == nil {
            for k, v := range local.Groups {
                manifest.Groups[k] = v
            }
        }
    }
    
    return manifest, nil
}

// LoadProfilesByGroup loads agent profiles filtered by group
func LoadProfilesByGroup(sourceDir string, filter GroupFilter) (map[string]AgentProfile, error) {
    if filter.AllGroups {
        return LoadProfiles(sourceDir)
    }
    
    manifest, err := LoadGroupManifest(sourceDir)
    if err != nil {
        // Fallback: if groups.json is broken/missing, load all
        return LoadProfiles(sourceDir)
    }
    
    // Build set of allowed agent names
    allowed := make(map[string]bool)
    
    // Core is always included
    if core, ok := manifest.Groups["core"]; ok {
        for _, name := range core.Agents {
            allowed[name] = true
        }
    }
    
    // Add requested groups
    for _, groupName := range filter.Groups {
        if def, ok := manifest.Groups[groupName]; ok {
            for _, name := range def.Agents {
                allowed[name] = true
            }
        }
    }
    
    // Load all profiles, then filter
    allProfiles, err := LoadProfiles(sourceDir)
    if err != nil {
        return nil, err
    }
    
    result := make(map[string]AgentProfile)
    for name, profile := range allProfiles {
        if allowed[name] {
            result[name] = profile
        }
    }
    
    return result, nil
}

// ListGroups returns available group names from groups.json
func ListGroups(sourceDir string) ([]string, error) {
    manifest, err := LoadGroupManifest(sourceDir)
    if err != nil {
        return nil, err
    }
    var names []string
    for name := range manifest.Groups {
        names = append(names, name)
    }
    sort.Strings(names)
    return names, nil
}
```

### 5.3 MODIFY: `ywai/cmd/ywai/root.go`

**installAgentProfiles signature change**:

```go
// Before:
func installAgentProfiles(agents []agent.Agent, dryRun bool)

// After:
func installAgentProfiles(agents []agent.Agent, dryRun bool, filter agents.GroupFilter)
```

Inside the function, replace `LoadProfiles(sourceDir)` with:
```go
var profiles map[string]agents.AgentProfile
if filter.AllGroups || len(filter.Groups) == 0 && !hasCustomGroups(sourceDir) {
    profiles, err = agents.LoadProfiles(sourceDir)
} else {
    profiles, err = agents.LoadProfilesByGroup(sourceDir, filter)
}
```

**executeInstall signature change**:
```go
func executeInstall(opts gentlai.InstallOptions, installMCP bool, globalOnly bool, adoFlag bool, groupFilter agents.GroupFilter)
```

### 5.4 MODIFY: `ywai/cmd/ywai/commands.go`

**New flags on installCmd**:
```go
installCmd.Flags().StringSlice("group", []string{}, "Agent groups to install (repeatable)")
installCmd.Flags().Bool("all-groups", false, "Install all agent groups")
```

**New command: `ywai groups`**:
```go
var groupsCmd = &cobra.Command{
    Use:   "groups",
    Short: "List available agent groups",
    Run: func(cmd *cobra.Command, args []string) {
        sourceDir := config.DataAgentsDir()
        names, err := agents.ListGroups(sourceDir)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error: %v\n", err)
            os.Exit(1)
        }
        for _, name := range names {
            fmt.Println(name)
        }
    },
}
```

**Install Run function**: Extract group flags:
```go
groups := getStringSliceFlag(cmd, "group")
allGroups := getBoolFlag(cmd, "all-groups")
groupFilter := agents.GroupFilter{
    Groups:    groups,
    AllGroups: allGroups,
}
executeInstall(installOpts, installMCP, globalOnly, adoFlag, groupFilter)
```

### 5.5 TUI Integration: `ywai/internal/tui/`

In the interactive install TUI, the **Options step** gains group selection:

- After persona selection, before final confirmation
- Groups are listed as checkboxes with descriptions
- "core" is pre-checked and disabled (non-optional, always installed)
- Optional groups (social-refactor, etc.) are togglable
- Space to toggle, Enter to confirm
- Confirmation summary shows selected groups

**Bubble Tea model additions**:
```go
type installModel struct {
    // ... existing fields
    availableGroups []agents.GroupDefinition
    selectedGroups  map[string]bool // group name → selected
    groupCursor     int
}
```

## 6. CLI Design

```bash
# Install core only (default behavior)
ywai install

# Install core + social-refactor
ywai install --group social-refactor

# Install multiple optional groups
ywai install --group social-refactor --group future-group

# Install everything (backward compat / override)
ywai install --all-groups

# List available groups
ywai groups

# Dry run with group selection
ywai install --group social-refactor --dry-run
```

**Error handling**:
- Unknown group name → warning, skip it, continue with known groups
- Missing/corrupt groups.json → warn, fall back to loading all profiles
- Empty groups list (only core) → install core, no error

## 7. Migration Path

### Fresh install
- `ywai install` → only core group installed
- Users opt-in to additional groups via `--group` flag

### Existing install upgrade
- `ywai update` → existing agents already installed; no changes  
- Next `ywai install` → uses new groups system  
- If `groups.json` is missing (pre-existing extracted data), falls back to `LoadProfiles` (load all)

### Custom groups
- `groups.local.json` in `~/.local/share/ywai/agents/` (gitignored)
- Merged with base `groups.json` on load
- Survives updates (not overwritten by embedded data)

## 8. ADR — Architecture Decision Record

**Title**: Agent Groups via groups.json Manifest

**Status**: Proposed

**Context**:  
`ywai` currently ships 8 agent profiles installed unconditionally to every detected AI agent. As the ecosystem grows (more agent profiles, domain-specific agents like SocialRefactor), users need to opt into only what's relevant.

**Decision**:  
Use a `groups.json` manifest file in the `ywai/agents/` directory to define named groups of agent profiles. The file is embedded in the binary and extracted on install. The `core` group is always installed; optional groups are selected via `--group` CLI flag or TUI checkboxes.

**Alternatives considered**:
1. Directory-based grouping (`agents/core/`, `agents/social-refactor/`) — rejected due to file moves and path breakage.
2. Inline metadata in AGENT.md YAML frontmatter — rejected due to prompt pollution.
3. External config in `~/.config/ywai/` — rejected due to separation from source of truth.

**Consequences**:
- ✅ No file moves; backward compatible
- ✅ Simple JSON format, human-editable
- ✅ `groups.local.json` for user overrides
- ✅ TUI integration straightforward (checkboxes)
- ⚠️ New file to maintain when adding agent profiles
- ⚠️ Agent-to-group mapping is one-to-one (an agent can only be in one group)

## 9. Summary

| Aspect | Detail |
|---|---|
| New files | `groups.json` (±15 lines) |
| New Go code | ±220 lines across 4 files |
| Modified files | agents.go, root.go, commands.go, TUI model |
| Backward compat | Full: `LoadProfiles` unchanged, `--all-groups` flag |
| TUI impact | Checkboxes in Options step |
| Migration | Automatic via fallback logic |
