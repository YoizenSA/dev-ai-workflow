package control

import "testing"

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
