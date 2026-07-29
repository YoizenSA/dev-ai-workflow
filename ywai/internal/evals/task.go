// Package evals runs an agent against a fixed task on several models and scores
// what came back.
//
// Scoring is the point. Efficiency alone ranks a model that answers in eight turns
// and misses half the call sites above one that grinds to the complete answer, and
// measurements on this repo showed exactly that inversion — so every task carries the
// answer it expects, and tasks are versioned in the repo so a result stays comparable
// across runs and reviewable in a PR.
package evals

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed tasks/*.json
var builtinTasks embed.FS

// Expectation is one thing the answer must mention. Matching is a lower-cased
// substring test: robust to a model's formatting, and specific enough that naming a
// symbol is the only realistic way to hit it.
type Expectation struct {
	Needle string `json:"needle"`
	Label  string `json:"label"`
	// Hard marks the expectation that separates a real trace from a shallow one, so a
	// partial score can still say whether the model did the difficult part.
	Hard bool `json:"hard,omitempty"`
}

// Task is one benchmark: a brief for an agent, plus the answer it should produce.
type Task struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Agent       string        `json:"agent"`
	Description string        `json:"description,omitempty"`
	Brief       string        `json:"brief"`
	Expect      []Expectation `json:"expect"`
}

func (t Task) validate() error {
	switch {
	case strings.TrimSpace(t.ID) == "":
		return fmt.Errorf("task is missing an id")
	case strings.TrimSpace(t.Agent) == "":
		return fmt.Errorf("task %q is missing an agent", t.ID)
	case strings.TrimSpace(t.Brief) == "":
		return fmt.Errorf("task %q is missing a brief", t.ID)
	case len(t.Expect) == 0:
		return fmt.Errorf("task %q has no expectations, so a run could not be scored", t.ID)
	}
	return nil
}

// Score is how much of the expected answer a response contains.
type Score struct {
	Hits     []string `json:"hits"`
	Missed   []string `json:"missed"`
	Total    int      `json:"total"`
	GotHard  bool     `json:"gotHard"`
	Answered bool     `json:"answered"`
}

// Score checks a response against the task's expectations. An empty or errored
// response is recorded as unanswered rather than as a zero: a run that never
// produced text is a missing measurement, not a wrong answer.
func (t Task) Score(response string) Score {
	s := Score{Total: len(t.Expect), Hits: []string{}, Missed: []string{}}
	trimmed := strings.TrimSpace(response)
	s.Answered = len(trimmed) >= 20 && !strings.HasPrefix(trimmed, "ERROR:")
	low := strings.ToLower(response)
	for _, e := range t.Expect {
		label := e.Label
		if label == "" {
			label = e.Needle
		}
		if s.Answered && strings.Contains(low, strings.ToLower(e.Needle)) {
			s.Hits = append(s.Hits, label)
			if e.Hard {
				s.GotHard = true
			}
			continue
		}
		s.Missed = append(s.Missed, label)
	}
	return s
}

// LoadTasks returns the built-in tasks, with any task in <projectDir>/ywai/evals/tasks
// overriding a built-in of the same id so a project can adapt one without a release.
func LoadTasks(projectDir string) ([]Task, error) {
	byID := map[string]Task{}

	entries, err := fs.ReadDir(builtinTasks, "tasks")
	if err != nil {
		return nil, fmt.Errorf("read built-in tasks: %w", err)
	}
	for _, e := range entries {
		raw, err := fs.ReadFile(builtinTasks, filepath.Join("tasks", e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		t, err := decodeTask(raw, e.Name())
		if err != nil {
			return nil, err
		}
		byID[t.ID] = t
	}

	if dir := strings.TrimSpace(projectDir); dir != "" {
		local := filepath.Join(dir, "ywai", "evals", "tasks")
		if matches, _ := filepath.Glob(filepath.Join(local, "*.json")); len(matches) > 0 {
			for _, p := range matches {
				raw, err := os.ReadFile(p)
				if err != nil {
					continue // an unreadable local task must not hide the built-ins
				}
				t, err := decodeTask(raw, filepath.Base(p))
				if err != nil {
					continue
				}
				byID[t.ID] = t
			}
		}
	}

	out := make([]Task, 0, len(byID))
	for _, t := range byID {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func decodeTask(raw []byte, name string) (Task, error) {
	var t Task
	if err := json.Unmarshal(raw, &t); err != nil {
		return Task{}, fmt.Errorf("parse %s: %w", name, err)
	}
	if err := t.validate(); err != nil {
		return Task{}, fmt.Errorf("%s: %w", name, err)
	}
	return t, nil
}

// FindTask returns the task with the given id.
func FindTask(projectDir, id string) (Task, error) {
	tasks, err := LoadTasks(projectDir)
	if err != nil {
		return Task{}, err
	}
	for _, t := range tasks {
		if t.ID == id {
			return t, nil
		}
	}
	return Task{}, fmt.Errorf("unknown task %q", id)
}
