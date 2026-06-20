//go:build !windows

package selfupdate

import "fmt"

// deferredReplace is a no-op on Unix where renaming a running binary works.
// The call site guards with runtime.GOOS == "windows" so this should never
// be reached, but the stub satisfies the compiler.
func deferredReplace(newBinary, exePath, version string) (string, error) {
	return "", fmt.Errorf("deferred replace not available on this platform")
}
