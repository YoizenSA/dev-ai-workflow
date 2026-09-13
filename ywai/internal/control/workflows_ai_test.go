package control

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestExtractJSONObject(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", `{"a":1}`, `{"a":1}`},
		{"fenced", "```json\n{\"a\":1}\n```", `{"a":1}`},
		{"noise around", "> orchestrator · model\n{\"a\":1}\nDone.", `{"a":1}`},
		{"nested", `prefix {"a":{"b":2}} suffix`, `{"a":{"b":2}}`},
		{"none", "no json here", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := extractJSONObject(c.in); got != c.want {
				t.Errorf("extractJSONObject(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestClassifyAIEditError(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		timedOut bool
		model    string
		stderr   string
		wantSub  string
	}{
		{
			name:    "no balance names the model and the way out",
			err:     errors.New("exit status 1"),
			model:   "zai/glm-5.3-flash",
			stderr:  "Error: Insufficient balance or no resource package. Please recharge.",
			wantSub: "no balance left",
		},
		{
			name:    "unavailable model",
			err:     errors.New("exit status 1"),
			model:   "opencode-admin/mimo-v2.5-free",
			stderr:  "ModelUnavailableError: Model unavailable: opencode-admin/mimo-v2.5-free",
			wantSub: "not available on this account",
		},
		{
			name:    "rejected api key",
			err:     errors.New("exit status 1"),
			model:   "openai/gpt-5.4-mini",
			stderr:  "Error: Incorrect API key provided: thk_live***. You can find your API key at https://platform.openai.com/account/api-keys.",
			wantSub: "API key",
		},
		{
			name:    "bare model id without provider",
			err:     errors.New("exit status 1"),
			model:   "glm-5.3-flash",
			stderr:  "Error: Invalid model reference: glm-5.3-flash",
			wantSub: "not a valid model reference",
		},
		{
			name:     "timeout suggests smaller edit",
			timedOut: true,
			err:      context.DeadlineExceeded,
			model:    "opencode-go/glm-5.3-flash",
			wantSub:  "took longer than",
		},
		{
			name:    "unknown failure keeps the technical detail",
			err:     errors.New("exit status 1"),
			model:   "",
			stderr:  "something exotic",
			wantSub: "opencode run failed",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := classifyAIEditError(c.err, c.timedOut, c.model, c.stderr).Error()
			if !strings.Contains(got, c.wantSub) {
				t.Errorf("classifyAIEditError() = %q, want it to contain %q", got, c.wantSub)
			}
		})
	}
}
