package missions

import (
	"errors"
	"testing"
)

func TestLogTail(t *testing.T) {
	short := "hello"
	if got := logTail(short); got != short {
		t.Errorf("short string mutated: %q", got)
	}
	big := make([]byte, logTailMax*2)
	for i := range big {
		big[i] = 'a'
	}
	got := logTail(string(big))
	if len(got) != logTailMax {
		t.Errorf("expected %d bytes, got %d", logTailMax, len(got))
	}
}

func TestIsRetriableModelError_FromLogTail(t *testing.T) {
	// Simulates what the worker chain does: classify the tail of captured stdout/stderr.
	logSnippet := "...lots of noise...\n429 rate limit exceeded for model x\n"
	if !isRetriableModelError(errors.New(logSnippet)) {
		t.Fatalf("rate limit log tail should be retriable")
	}
	authSnippet := "401 unauthorized\nplease set ANTHROPIC_API_KEY"
	if isRetriableModelError(errors.New(authSnippet)) {
		t.Fatalf("auth error should NOT be retriable")
	}
}
