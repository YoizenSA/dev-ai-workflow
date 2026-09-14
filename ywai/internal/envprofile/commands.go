package envprofile

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Exported workflows live in the env as opencode slash commands:
// <config>/opencode/commands/<workflow>.md, written by the workflow exporter.
// Their frontmatter names the orchestrator agent to run, which is what
// `ywai <env> <workflow> "task"` needs.

// CommandsDir is the env's opencode commands dir.
func CommandsDir(p Profile) string {
	return filepath.Join(profileDir(p.Name), "config", "opencode", "commands")
}

// Commands lists the command names exported into an env, sorted.
func Commands(p Profile) []string {
	entries, err := os.ReadDir(CommandsDir(p))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if name, ok := strings.CutSuffix(e.Name(), ".md"); ok {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// CommandRun resolves an exported command into what a headless run needs:
// agent is the frontmatter `agent:` value (the workflow orchestrator; empty
// for a plain prompt command), and body is the markdown after the frontmatter
// (the prompt template, with $ARGUMENTS still in place). ok is false when the
// env has no such command.
func CommandRun(p Profile, name string) (agent, body string, ok bool) {
	name = strings.TrimSpace(strings.TrimPrefix(name, "/"))
	if name == "" || strings.ContainsAny(name, `/\`) {
		return "", "", false
	}
	data, err := os.ReadFile(filepath.Join(CommandsDir(p), name+".md"))
	if err != nil {
		return "", "", false
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		// No frontmatter: the whole file is the prompt.
		return "", strings.TrimSpace(string(data)), true
	}
	end := -1
	for i, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			end = i + 1
			break
		}
		if v, has := strings.CutPrefix(line, "agent:"); has {
			agent = strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	if end < 0 {
		// Unterminated frontmatter: nothing usable beyond it.
		return agent, "", true
	}
	return agent, strings.TrimSpace(strings.Join(lines[end+1:], "\n")), true
}
