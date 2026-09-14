package envprofile

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// PidFile is <profile>/run/service.pid.
func PidFile(p Profile) string {
	return filepath.Join(profileDir(p.Name), "run", "service.pid")
}

// CredsFile is <profile>/run/creds.env (0600): per-profile server credentials.
func CredsFile(p Profile) string {
	return filepath.Join(profileDir(p.Name), "run", "creds.env")
}

// ServerLog is the profile-scoped opencode log.
func ServerLog(p Profile) string {
	return filepath.Join(profileDir(p.Name), "data", "opencode", "log", "opencode.log")
}

// serverUsername is the fixed username opencode2 serve authenticates: like
// openCodeChildEnv in cmd/ywai/commands.go, the username is always
// "opencode" and only the password is secret. A custom username 401s even
// with the right password.
const serverUsername = "opencode"

// EnsureCreds generates per-profile server credentials once and returns them.
// Stored 0600: one more reason profiles never share a directory. Credentials
// stored for any other username are regenerated: older builds wrote
// per-profile usernames that the server rejects.
func EnsureCreds(p Profile) (user, pass string, err error) {
	if user, pass, ok := readCreds(p); ok && user == serverUsername {
		return user, pass, nil
	}
	user = serverUsername
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate server password: %w", err)
	}
	pass = hex.EncodeToString(buf)
	content := "OPENCODE_SERVER_USERNAME=" + user + "\nOPENCODE_SERVER_PASSWORD=" + pass + "\n"
	if err := os.WriteFile(CredsFile(p), []byte(content), 0o600); err != nil {
		return "", "", fmt.Errorf("write server creds: %w", err)
	}
	return user, pass, nil
}

// readCreds parses the stored server credentials without creating them.
// ok is false when the profile never started (no creds file yet).
func readCreds(p Profile) (user, pass string, ok bool) {
	data, err := os.ReadFile(CredsFile(p))
	if err != nil {
		return "", "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if v, found := strings.CutPrefix(line, "OPENCODE_SERVER_USERNAME="); found {
			user = strings.TrimSpace(v)
		}
		if v, found := strings.CutPrefix(line, "OPENCODE_SERVER_PASSWORD="); found {
			pass = strings.TrimSpace(v)
		}
	}
	if user == "" || pass == "" {
		return "", "", false
	}
	return user, pass, true
}

// healthOK probes url, retrying with the profile server credentials when the
// server answers 401: a credentialed server guards even /api/health, so an
// unauthenticated probe alone would report a healthy server as down.
func healthOK(client *http.Client, url, user, pass string) bool {
	if resp, err := client.Get(url); err == nil {
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return true
		}
		if resp.StatusCode != http.StatusUnauthorized || user == "" {
			return false
		}
	} else {
		return false
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	req.SetBasicAuth(user, pass)
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// profileCmdEnv merges the OS environment with the profile delta plus creds.
func profileCmdEnv(p Profile) ([]string, error) {
	user, pass, err := EnsureCreds(p)
	if err != nil {
		return nil, err
	}
	env := os.Environ()
	for k, v := range Env(p) {
		env = append(env, k+"="+v)
	}
	return append(env,
		"OPENCODE_SERVER_USERNAME="+user,
		"OPENCODE_SERVER_PASSWORD="+pass,
	), nil
}

// Start launches `opencode2 serve --port <port>` under the profile
// environment and records its pid. It returns once the server answers
// /api/health (or an error when the deadline passes).
func Start(ctx context.Context, p Profile, opencodeBin string) error {
	if running, err := Status(p); err == nil && running {
		return nil
	}
	// A live pid with failing health means stale state (rotated credentials,
	// crashed-then-reused pid): stop our own pid-file holder before binding
	// the port fresh. Only the pid we recorded is ever signaled.
	if raw, err := os.ReadFile(PidFile(p)); err == nil {
		if pid, perr := strconv.Atoi(strings.TrimSpace(string(raw))); perr == nil && PidAlive(pid) {
			_ = Stop(p)
		}
	}
	// Fast-fail when something already answers on our port (a stale server
	// no pidfile claims): launching now would bind-fail while health polls
	// burn the 30s deadline against another process's responses.
	url := Env(p)["OPENCODE_URL"]
	if portOccupied(url) {
		return fmt.Errorf("port %d already serves an opencode server no profile claims (stale process?). Free it and retry Start", p.Port)
	}
	if err := Ensure(p); err != nil {
		return err
	}
	env, err := profileCmdEnv(p)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, opencodeBin, "serve", "--port", strconv.Itoa(p.Port))
	cmd.Env = env
	// Detach output: the server owns its log file under the profile.
	logPath := ServerLog(p)
	_ = os.MkdirAll(filepath.Dir(logPath), 0o700)
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open server log: %w", err)
	}
	defer logFile.Close()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start profile server: %w", err)
	}
	// Detach so the child survives this process; the pid file owns it.
	_ = cmd.Process.Release()
	if err := os.WriteFile(PidFile(p), []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}
	return waitHealthy(ctx, p)
}

// Stop kills the profile server recorded in its pid file and waits until
// its port stops answering (up to ~5s): on Windows a killed process can
// hold the database/log files briefly, and deleting the directory before
// the handles release fails the whole removal. Missing or stale pid files
// are success (already stopped).
func Stop(p Profile) error {
	raw, err := os.ReadFile(PidFile(p))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read pid file: %w", err)
	}
	pid := 0
	if err == nil {
		if v, perr := strconv.Atoi(strings.TrimSpace(string(raw))); perr == nil && v > 0 {
			pid = v
		} else {
			// Unparseable or non-positive pid file can never name a live
			// server (PID -1 on unix names a process group — never signal
			// it). Drop the garbage so later reads see "no server".
			_ = os.Remove(PidFile(p))
		}
	}
	if pid > 0 {
		if proc, ferr := os.FindProcess(pid); ferr == nil {
			_ = proc.Kill()
		}
		_ = os.Remove(PidFile(p))
	}
	// The server forks: the pidfile may be missing or name a dead launcher
	// while its child still listens. Reap a listener on our port, but only
	// when its image is an opencode binary — foreign processes are never
	// signaled.
	if lp := listenerPID(p.Port); lp != 0 && lp != pid && isOpencodeProcess(lp) {
		if proc, ferr := os.FindProcess(lp); ferr == nil {
			_ = proc.Kill()
		}
	}
	waitPortFree(Env(p)["OPENCODE_URL"], 5*time.Second)
	return nil
}

// waitPortFree polls until nothing answers on url (any status counts) or
// the timeout passes. It never fails: callers treat a lingering listener
// as a delete-time conflict, not a stop error.
func waitPortFree(url string, timeout time.Duration) {
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url + "/api/health")
		if err != nil {
			return
		}
		resp.Body.Close()
		time.Sleep(250 * time.Millisecond)
	}
}

// Status reports whether the profile server answers its health endpoint.
// A stale pid file alone never counts as running.
func Status(p Profile) (bool, error) {
	if _, err := os.Stat(PidFile(p)); err != nil {
		return false, nil
	}
	client := &http.Client{Timeout: 2 * time.Second}
	user, pass, _ := readCreds(p)
	return healthOK(client, Env(p)["OPENCODE_URL"]+"/api/health", user, pass), nil
}

// PidAlive reports whether pid is a live process (unix signal 0; on Windows
// any existing pid handle counts, so callers prefer Status). Non-positive
// pids are never alive: signal 0 on PID -1/0 addresses a process group.
func PidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// portOccupied reports whether anything answers on the profile URL, healthy
// or not (even a 401 proves a listener). Connection-refused means free.
func portOccupied(url string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url + "/api/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return true
}

func waitHealthy(ctx context.Context, p Profile) error {
	deadline := time.Now().Add(30 * time.Second)
	url := Env(p)["OPENCODE_URL"] + "/api/health"
	user, pass, _ := readCreds(p)
	client := &http.Client{Timeout: 2 * time.Second}
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		if user != "" {
			req.SetBasicAuth(user, pass)
		}
		if resp, err := client.Do(req); err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("profile server %q did not answer %s in time", p.Name, url)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}
