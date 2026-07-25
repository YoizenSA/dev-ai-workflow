package web

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
)

type countingModelsClient struct {
	fakeOpencodeClient
	calls atomic.Int32
	delay time.Duration
}

func (c *countingModelsClient) ListModels(ctx context.Context) ([]opencode.ModelInfo, error) {
	c.calls.Add(1)
	if c.delay > 0 {
		select {
		case <-time.After(c.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return []opencode.ModelInfo{{ID: "p/m", Name: "m", Provider: "p"}}, nil
}

func TestWarmModelsPrefillsCache(t *testing.T) {
	client := &countingModelsClient{}
	h := &Handlers{opencodeClient: client}
	h.modelCache.path = filepath.Join(t.TempDir(), "models.json")

	h.WarmModels()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		h.modelCache.mu.Lock()
		ready := !h.modelCache.fetchedAt.IsZero() && len(h.modelCache.models) == 1
		h.modelCache.mu.Unlock()
		if ready {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	h.modelCache.mu.Lock()
	defer h.modelCache.mu.Unlock()
	if h.modelCache.fetchedAt.IsZero() {
		t.Fatal("expected WarmModels to populate fetchedAt")
	}
	if len(h.modelCache.models) != 1 {
		t.Fatalf("expected 1 cached model, got %d", len(h.modelCache.models))
	}
	if client.calls.Load() < 1 {
		t.Fatal("expected ListModels to be called during warm")
	}
	// Disk should be written after a successful warm.
	if _, err := os.Stat(h.modelCache.path); err != nil {
		t.Fatalf("expected disk cache file: %v", err)
	}
}

func TestWarmModelsIsIdempotent(t *testing.T) {
	client := &countingModelsClient{delay: 50 * time.Millisecond}
	h := &Handlers{opencodeClient: client}
	h.modelCache.path = filepath.Join(t.TempDir(), "models.json")

	h.WarmModels()
	h.WarmModels()
	h.WarmModels()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if client.calls.Load() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	if got := client.calls.Load(); got != 1 {
		t.Fatalf("expected single ListModels call from concurrent WarmModels, got %d", got)
	}
}

func TestModelCacheSingleflightCold(t *testing.T) {
	client := &countingModelsClient{delay: 80 * time.Millisecond}
	cache := &modelCache{path: filepath.Join(t.TempDir(), "models.json")}

	const n = 8
	var wg sync.WaitGroup
	errs := make(chan error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			models, err := cache.get(context.Background(), client.ListModels)
			if err != nil {
				errs <- err
				return
			}
			if len(models) != 1 {
				errs <- errLen(len(models))
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := client.calls.Load(); got != 1 {
		t.Fatalf("expected 1 CLI call for concurrent cold gets, got %d", got)
	}
}

type errLen int

func (e errLen) Error() string { return "unexpected model count" }

func TestModelCacheDiskSeedSkipsCLI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "models.json")
	env := modelDiskEnvelope{
		FetchedAt: time.Now(),
		Models:    []opencode.ModelInfo{{ID: "disk/m", Name: "from-disk", Provider: "disk"}},
	}
	data, _ := json.Marshal(env)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	client := &countingModelsClient{}
	cache := &modelCache{path: path}

	models, err := cache.get(context.Background(), client.ListModels)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ID != "disk/m" {
		t.Fatalf("expected disk-seeded model, got %+v", models)
	}
	if client.calls.Load() != 0 {
		t.Fatalf("fresh disk seed must not call CLI, got %d calls", client.calls.Load())
	}
}

func TestModelCacheStaleDiskStillServesAndRefreshes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "models.json")
	env := modelDiskEnvelope{
		FetchedAt: time.Now().Add(-modelCacheTTL - time.Minute),
		Models:    []opencode.ModelInfo{{ID: "old/m", Name: "stale", Provider: "old"}},
	}
	data, _ := json.Marshal(env)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	client := &countingModelsClient{delay: 30 * time.Millisecond}
	cache := &modelCache{path: path}

	// First get: serve disk immediately (SWR), kick background refresh.
	models, err := cache.get(context.Background(), client.ListModels)
	if err != nil {
		t.Fatal(err)
	}
	if models[0].ID != "old/m" {
		t.Fatalf("expected stale disk served immediately, got %s", models[0].ID)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if client.calls.Load() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if client.calls.Load() < 1 {
		t.Fatal("expected background refresh after stale disk seed")
	}

	// Wait for refresh to land.
	for time.Now().Before(deadline) {
		cache.mu.Lock()
		id := ""
		if len(cache.models) > 0 {
			id = cache.models[0].ID
		}
		cache.mu.Unlock()
		if id == "p/m" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected memory to refresh to live model p/m")
}

func TestKickRefreshForcesCLIEvenWhenFresh(t *testing.T) {
	client := &countingModelsClient{delay: 20 * time.Millisecond}
	cache := &modelCache{path: filepath.Join(t.TempDir(), "models.json")}

	// Populate cache once.
	if _, err := cache.get(context.Background(), client.ListModels); err != nil {
		t.Fatal(err)
	}
	if client.calls.Load() != 1 {
		t.Fatalf("setup: want 1 call, got %d", client.calls.Load())
	}

	// Fresh within TTL: normal get must not hit CLI again.
	if _, err := cache.get(context.Background(), client.ListModels); err != nil {
		t.Fatal(err)
	}
	if client.calls.Load() != 1 {
		t.Fatalf("fresh get should not CLI, got %d calls", client.calls.Load())
	}

	// Explicit kick (Settings open) must revalidate in background.
	cache.kickRefresh(client.ListModels)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if client.calls.Load() >= 2 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("kickRefresh should force CLI; calls=%d", client.calls.Load())
}

func TestWarmModelsUsesDiskWithoutCLIWhenFresh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "models.json")
	env := modelDiskEnvelope{
		FetchedAt: time.Now(),
		Models:    []opencode.ModelInfo{{ID: "disk/m", Name: "from-disk", Provider: "disk"}},
	}
	data, _ := json.Marshal(env)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	client := &countingModelsClient{}
	h := &Handlers{opencodeClient: client}
	h.modelCache.path = path

	h.WarmModels()
	time.Sleep(50 * time.Millisecond)

	if client.calls.Load() != 0 {
		t.Fatalf("fresh disk must skip warm CLI, got %d calls", client.calls.Load())
	}
	h.modelCache.mu.Lock()
	defer h.modelCache.mu.Unlock()
	if len(h.modelCache.models) != 1 || h.modelCache.models[0].ID != "disk/m" {
		t.Fatalf("expected disk models after warm seed, got %+v", h.modelCache.models)
	}
}
