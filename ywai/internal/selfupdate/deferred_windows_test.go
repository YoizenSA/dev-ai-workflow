//go:build windows

package selfupdate

import "testing"

func TestDetachedCmdArgs(t *testing.T) {
	const batPath = `C:\Users\u\AppData\Local\Temp\ywai.update.bat`
	name, args := detachedCmdArgs(batPath)
	if name != "cmd" {
		t.Errorf("name = %q, want %q", name, "cmd")
	}
	want := []string{"/C", "start", "", "/B", batPath}
	if len(args) != len(want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("args[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}
