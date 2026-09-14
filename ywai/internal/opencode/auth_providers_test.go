package opencode

import "testing"

func TestParseAuthedProviders(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "live shape",
			in: `[
				{"id":"opencode-go","name":"OpenCode Go","connections":[{"type":"credential","id":"cred_1","label":"default"}]},
				{"id":"meta","name":"Meta","connections":[{"type":"credential","id":"cred_2","label":"Meta"}]}
			]`,
			want: []string{"opencode-go", "meta"},
		},
		{
			name: "provider without connections is skipped",
			in:   `[{"id":"tokenharbor","name":"Token Harbor","connections":[]},{"id":"zai","name":"Z.AI","connections":[{"id":"c"}]}]`,
			want: []string{"zai"},
		},
		{
			name: "empty id is skipped",
			in:   `[{"name":"no-id","connections":[{"id":"c"}]}]`,
			want: []string{},
		},
		{
			name: "not json yields nil so callers degrade to no filtering",
			in:   `ERROR: service unreachable`,
			want: nil,
		},
		{
			name: "empty list",
			in:   `[]`,
			want: []string{},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseAuthedProviders([]byte(c.in))
			if c.want == nil {
				if got != nil {
					t.Fatalf("parseAuthedProviders() = %v, want nil", got)
				}
				return
			}
			if len(got) != len(c.want) {
				t.Fatalf("parseAuthedProviders() = %v, want %v", got, c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Fatalf("parseAuthedProviders() = %v, want %v", got, c.want)
				}
			}
		})
	}
}
