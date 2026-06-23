package mcp

// discovery_edges_test.go — TDD slice 1, validate phase.
//
// Edge-case tests for the four uncovered branches in discovery.go that the
// 10 pinned contract tests in discovery_test.go do not exercise:
//
//  1. DiscoverStdio empty command — the input-validation early return at
//     L52-54. Any caller passing []string{} (e.g. a misconfigured install
//     manifest) hits this path; without a test it would silently slip
//     through code review as "unreachable" when it is in fact the FIRST
//     line of defense against bogus config.
//
//  2. readJSONRPCResponse skips empty lines — L242-243. Real stdio MCP
//     servers routinely emit blank separator lines; the parser must skip
//     them rather than return an unmarshal error on "".
//
//  3. readJSONRPCResponse skips malformed JSON and notifications — L246-247
//     + L250-251. A server that logs a non-JSON line to stdout (common when
//     stderr is redirected, or when the binary is wrapped in a launcher
//     that prints a banner) and then emits a notifications/* message
//     before its real response must NOT cause the probe to abort. The
//     helper is documented as a "skip until id" loop; these two branches
//     are the implementation of that contract.
//
//  4. readJSONRPCResponse 50-line budget — L255. A misbehaving server that
//     emits an infinite stream of notifications (or, worse, a
//     non-terminating log to stdout) would otherwise hang the probe for
//     the full 8s timeout. The 50-line cap is a hard ceiling; if the
//     budget runs out, we surface a clean error instead.
//
// All four are stdlib-only direct unit tests. They do NOT require the
// compiled-binary fake from fake_mcp_test.go, so they are cheap (no
// process spawn, no Go toolchain on the test path) and they exercise
// code paths the integration tests cannot reach without modifying the
// fake stub.

import (
	"bufio"
	"context"
	"strings"
	"testing"
)

// TestDiscoverStdio_EmptyCommand pins the input-validation early return.
// Passing a nil or zero-length command slice must yield a non-nil error
// whose message contains "empty command" — this is the contract the rest
// of ywai's install pipeline relies on to detect a misconfigured server.
//
// The test must NOT spawn any subprocess: the early return happens before
// exec.Command is called.
func TestDiscoverStdio_EmptyCommand(t *testing.T) {
	cases := []struct {
		name    string
		command []string
	}{
		{"nil slice", nil},
		{"empty slice", []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tools, err := DiscoverStdio(context.Background(), tc.command, nil)
			if err == nil {
				t.Fatalf("DiscoverStdio(%s) err = nil, tools = %v, want error",
					tc.name, tools)
			}
			if !strings.Contains(err.Error(), "empty command") {
				t.Errorf("DiscoverStdio(%s) err = %q, want to mention \"empty command\"",
					tc.name, err)
			}
			if tools != nil {
				t.Errorf("DiscoverStdio(%s) tools = %v, want nil on error",
					tc.name, tools)
			}
		})
	}
}

// TestReadJSONRPCResponse_SkipsEmptyLines pins the "skip blank lines"
// branch. The helper calls strings.TrimSpace and continues on empty
// results; without this branch, the very first separator line from a
// well-behaved stdio MCP server would surface as a json unmarshal error.
//
// The input is a sequence of blank/whitespace-only lines followed by a
// valid response. We assert the response is returned with its id intact.
func TestReadJSONRPCResponse_SkipsEmptyLines(t *testing.T) {
	input := "\n\n   \n\t\n" +
		`{"jsonrpc":"2.0","id":42,"result":{"ok":true}}` + "\n"

	resp, err := readJSONRPCResponse(bufio.NewReader(strings.NewReader(input)))
	if err != nil {
		t.Fatalf("readJSONRPCResponse err = %v, want nil", err)
	}
	// JSON numbers decode into map[string]interface{} as float64.
	id, ok := resp["id"].(float64)
	if !ok {
		t.Fatalf("readJSONRPCResponse resp[id] type = %T, want float64", resp["id"])
	}
	if id != 42 {
		t.Errorf("readJSONRPCResponse resp[id] = %v, want 42", id)
	}
}

// TestReadJSONRPCResponse_SkipsMalformedJSONAndNotifications pins both
// the "skip on unmarshal error" branch (L246-247) and the "skip when
// response has no id field" branch (L250-251). The helper's documented
// contract is: keep reading until you find a JSON object that has an id
// (i.e. a response to a request, not a server-initiated notification).
//
// Real-world input the probe must survive:
//
//	not even close to JSON
//	{"jsonrpc":"2.0","method":"notifications/message","params":{...}}
//	{"jsonrpc":"2.0","id":7,"result":{"tools":[]}}
//
// The first two lines must be skipped; the third is the real response.
func TestReadJSONRPCResponse_SkipsMalformedJSONAndNotifications(t *testing.T) {
	input := strings.Join([]string{
		`not even close to JSON`,
		`{"jsonrpc":"2.0","method":"notifications/message","params":{"level":"info"}}`,
		`{"jsonrpc":"2.0","id":7,"result":{"tools":[]}}`,
		"",
	}, "\n")

	resp, err := readJSONRPCResponse(bufio.NewReader(strings.NewReader(input)))
	if err != nil {
		t.Fatalf("readJSONRPCResponse err = %v, want nil (must skip noise lines)", err)
	}
	id, ok := resp["id"].(float64)
	if !ok {
		t.Fatalf("readJSONRPCResponse resp[id] type = %T, want float64", resp["id"])
	}
	if id != 7 {
		t.Errorf("readJSONRPCResponse resp[id] = %v, want 7", id)
	}
	// And the response must be the one with a result, not the notification
	// that preceded it.
	if _, hasResult := resp["result"]; !hasResult {
		t.Errorf("readJSONRPCResponse resp = %v, want the response (with result), "+
			"not the notification we were supposed to skip", resp)
	}
}

// TestReadJSONRPCResponse_BudgetExhausted pins the 50-line safety cap.
// A misbehaving server emitting an unbounded stream of notifications
// would otherwise hang the probe for the full 8s safety timeout. The
// 50-line cap turns that into a clean, fast error.
//
// We feed 50 lines, alternating empty and notification-shaped, so the
// loop runs to its full budget without ever finding a response with an
// id. The function must return an error mentioning "50 lines" — this is
// the discriminator callers can use to distinguish a "server is broken"
// situation from a "server is slow" one (the latter is the 8s ctx
// timeout's job).
func TestReadJSONRPCResponse_BudgetExhausted(t *testing.T) {
	var b strings.Builder
	b.Grow(50 * 80)
	for i := 0; i < 50; i++ {
		if i%2 == 0 {
			b.WriteByte('\n') // empty line → continue via L242-243
		} else {
			b.WriteString(`{"jsonrpc":"2.0","method":"notifications/log","params":{}}`)
			b.WriteByte('\n')
		}
	}

	_, err := readJSONRPCResponse(bufio.NewReader(strings.NewReader(b.String())))
	if err == nil {
		t.Fatal("readJSONRPCResponse(50 noise lines) err = nil, want budget-exhausted error")
	}
	if !strings.Contains(err.Error(), "50 lines") {
		t.Errorf("readJSONRPCResponse budget err = %q, want to mention \"50 lines\"", err)
	}
}
