package main

import (
	"errors"
	"testing"
)

// stubServerSeams replaces the package-level health-check/launch seams for one
// test and restores them on cleanup. Must never run in parallel: the seams are
// package globals shared by every test in this package.
func stubServerSeams(t *testing.T, port int, launchErr error) *bool {
	t.Helper()
	origPort, origLaunch := detectRunningServer, launchControlServer
	t.Cleanup(func() {
		detectRunningServer, launchControlServer = origPort, origLaunch
	})
	launched := false
	detectRunningServer = func() int { return port }
	launchControlServer = func() error {
		launched = true
		return launchErr
	}
	return &launched
}

func TestEnsureControlServerRunning_StartsWhenAbsent(t *testing.T) {
	launched := stubServerSeams(t, 0, nil)

	var r applyResult
	ensureControlServerRunning(&r, false)

	if !*launched {
		t.Fatal("expected server launch when nothing is running")
	}
	if len(r.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", r.Warnings)
	}
}

func TestEnsureControlServerRunning_SkipsWhenHealthy(t *testing.T) {
	launched := stubServerSeams(t, 5768, nil)

	var r applyResult
	ensureControlServerRunning(&r, false)

	if *launched {
		t.Fatal("expected no launch when a healthy server is already running")
	}
}

func TestEnsureControlServerRunning_DryRunDoesNotLaunch(t *testing.T) {
	launched := stubServerSeams(t, 0, nil)

	var r applyResult
	ensureControlServerRunning(&r, true)

	if *launched {
		t.Fatal("dry run must not launch the control server")
	}
}

func TestEnsureControlServerRunning_StartFailureWarns(t *testing.T) {
	launched := stubServerSeams(t, 0, errors.New("boom"))

	var r applyResult
	ensureControlServerRunning(&r, false)

	if !*launched {
		t.Fatal("expected launch attempt to be made")
	}
	if len(r.Warnings) != 1 {
		t.Fatalf("expected one warning, got %v", r.Warnings)
	}
}
