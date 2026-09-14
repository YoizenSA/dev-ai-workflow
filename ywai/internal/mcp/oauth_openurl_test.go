package mcp

import (
	"runtime"
	"testing"
)

func TestOpenURLCmdFor(t *testing.T) {
	const u = "https://example.com/auth?a=1&b=2"

	tests := []struct {
		goos    string
		wantCmd string
		want    []string
	}{
		{"darwin", "open", []string{u}},
		{"windows", "rundll32", []string{"url.dll,FileProtocolHandler", u}},
		{"linux", "xdg-open", []string{u}},
		{"freebsd", "xdg-open", []string{u}},
	}
	for _, tt := range tests {
		gotCmd, gotArgs := openURLCmdFor(tt.goos, u)
		if gotCmd != tt.wantCmd {
			t.Errorf("openURLCmdFor(%q) cmd = %q, want %q", tt.goos, gotCmd, tt.wantCmd)
		}
		if len(gotArgs) != len(tt.want) {
			t.Errorf("openURLCmdFor(%q) args = %v, want %v", tt.goos, gotArgs, tt.want)
			continue
		}
		for i := range tt.want {
			if gotArgs[i] != tt.want[i] {
				t.Errorf("openURLCmdFor(%q) args[%d] = %q, want %q", tt.goos, i, gotArgs[i], tt.want[i])
			}
		}
	}
}

func TestOpenURLCmdUsesHostGOOS(t *testing.T) {
	wantCmd, _ := openURLCmdFor(runtime.GOOS, "https://example.com")
	gotCmd, _ := openURLCmd("https://example.com")
	if gotCmd != wantCmd {
		t.Errorf("openURLCmd cmd = %q, want %q for GOOS %q", gotCmd, wantCmd, runtime.GOOS)
	}
}
