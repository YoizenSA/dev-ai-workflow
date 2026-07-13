package fastfs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootRejectsEscape(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Resolve("../outside"); err == nil {
		t.Fatal("expected escape rejection")
	}
	// Compare against r.Abs (symlink-resolved), not t.TempDir(): on macOS
	// /var/folders is a symlink to /private/var/folders.
	p, err := r.Resolve("ok.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !isUnder(r.Abs, p) {
		t.Fatalf("resolved %q not under root %q", p, r.Abs)
	}
}

func TestCacheHitMiss(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := NewFileCache(1 << 20)
	b1, err := c.Get(path)
	if err != nil || string(b1) != "hello" {
		t.Fatalf("first get: %v %q", err, b1)
	}
	b2, err := c.Get(path)
	if err != nil || string(b2) != "hello" {
		t.Fatalf("second get: %v %q", err, b2)
	}
	hits, misses, _, _ := c.Stats()
	if misses != 1 || hits != 1 {
		t.Fatalf("hits=%d misses=%d want 1/1", hits, misses)
	}
	// mutate → miss
	if err := os.WriteFile(path, []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}
	b3, err := c.Get(path)
	if err != nil || string(b3) != "world" {
		t.Fatalf("after write: %v %q", err, b3)
	}
	hits, misses, _, _ = c.Stats()
	if misses != 2 || hits != 1 {
		t.Fatalf("after invalidate hits=%d misses=%d", hits, misses)
	}
}

func TestFindAndSearch(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "a.go"), []byte("package pkg\n\nfunc Hello() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "b.txt"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "node_modules", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "node_modules", "x", "c.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc, err := NewService(dir)
	if err != nil {
		t.Fatal(err)
	}
	found, err := svc.Find(FindOptions{Pattern: "*.go", Max: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].Path != "pkg/a.go" {
		t.Fatalf("find = %+v", found)
	}

	res, err := svc.Search(SearchOptions{Pattern: `func Hello`, MaxMatches: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 1 {
		t.Fatalf("matches = %+v", res.Matches)
	}
	// warm search should hit cache
	res2, err := svc.Search(SearchOptions{Pattern: `func Hello`, MaxMatches: 10})
	if err != nil {
		t.Fatal(err)
	}
	if res2.CacheHits < 1 {
		t.Fatalf("expected cache hit on second search, got hits=%d misses=%d", res2.CacheHits, res2.CacheMisses)
	}
}

func TestOutlineAndSlice(t *testing.T) {
	dir := t.TempDir()
	body := "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := NewService(dir)
	if err != nil {
		t.Fatal(err)
	}
	o, err := svc.ReadOutline("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if o.Language != "go" || len(o.Signatures) == 0 {
		t.Fatalf("outline = %+v", o)
	}
	sl, err := svc.ReadSlice("main.go", 1, 2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sl.Content, "package main") {
		t.Fatalf("slice = %q", sl.Content)
	}
}

func TestMCPToolsListAndSearch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\nfunc Z(){}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := NewMCPAdapter(dir)
	if err != nil {
		t.Fatal(err)
	}
	resp := m.handle(rpcRequest{
		JSONRPC: "2.0",
		ID:      []byte("1"),
		Method:  "tools/list",
	})
	if resp == nil || resp.Result == nil {
		t.Fatal("tools/list failed")
	}
	raw, err := json.Marshal(map[string]interface{}{
		"name": "fastfs_search",
		"arguments": map[string]interface{}{
			"pattern": "func Z",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp = m.handle(rpcRequest{
		JSONRPC: "2.0",
		ID:      []byte("2"),
		Method:  "tools/call",
		Params:  raw,
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/call error: %+v", resp)
	}
}
