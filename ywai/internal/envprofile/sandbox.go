package envprofile

import (
	"os"
	"sort"
	"sync"
)

// sandboxMu serializes profile-env application. The sandbox mutates process
// state (os.Setenv), so profile-scoped applies must never run concurrently
// in one process. Web-triggered applies must shell out to the ywai binary
// instead of using this in-process sandbox.
var sandboxMu sync.Mutex

type savedEnv struct {
	value string
	ok    bool
}

// WithProfileEnv runs fn with the profile environment applied, restoring the
// previous process environment afterwards (including unsetting vars that
// were absent before). Do not nest calls.
func WithProfileEnv(p Profile, fn func() error) error {
	sandboxMu.Lock()
	defer sandboxMu.Unlock()

	delta := Env(p)
	prev := make(map[string]savedEnv, len(delta))
	keys := make([]string, 0, len(delta))
	for k := range delta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v, ok := os.LookupEnv(k)
		prev[k] = savedEnv{value: v, ok: ok}
		if err := os.Setenv(k, delta[k]); err != nil {
			restoreEnv(prev)
			return err
		}
	}
	defer restoreEnv(prev)
	return fn()
}

func restoreEnv(prev map[string]savedEnv) {
	for k, s := range prev {
		if s.ok {
			_ = os.Setenv(k, s.value)
		} else {
			_ = os.Unsetenv(k)
		}
	}
}
