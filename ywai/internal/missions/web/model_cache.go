package web

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
)

// modelCacheTTL is how long an in-memory list is considered fresh. Past this
// the previous list is still served immediately (SWR) while one background
// refresh runs. Settings entry always force-kicks a refresh regardless of TTL
// (see kickRefresh), so a long TTL only avoids accidental CLI spam — not
// staleness when the user is actively configuring models.
const modelCacheTTL = 6 * time.Hour

// modelCacheDiskMaxAge is the longest we will seed memory from disk without a
// live refresh having succeeded. Older files are ignored so a machine that
// has not run opencode in a long time does not show a permanently wrong list.
const modelCacheDiskMaxAge = 7 * 24 * time.Hour

// modelDiskFile is the basename under ~/.ywai/cache/.
const modelDiskFile = "opencode-models.json"

// modelCache memoizes the slow opencode model lookup with:
//   - in-memory stale-while-revalidate
//   - singleflight on cold (concurrent first requests share one CLI call)
//   - disk persistence so process restarts stay warm
//
// The zero value is ready to use (path defaults to ~/.ywai/cache/...).
type modelCache struct {
	mu         sync.Mutex
	models     []opencode.ModelInfo
	fetchedAt  time.Time
	refreshing bool
	// waiters receive the result of an in-flight cold fetch.
	waiters []chan coldResult
	// path overrides the disk file for tests. Empty → default under DataDir.
	path string
}

type coldResult struct {
	models []opencode.ModelInfo
	err    error
}

type modelDiskEnvelope struct {
	FetchedAt time.Time            `json:"fetchedAt"`
	Models    []opencode.ModelInfo `json:"models"`
}

func (c *modelCache) diskPath() string {
	if c.path != "" {
		return c.path
	}
	return filepath.Join(config.DataDir(), "cache", modelDiskFile)
}

// get returns cached models when available. A cold cache fetches once for all
// concurrent callers; a stale cache returns the old value immediately and
// refreshes in the background.
func (c *modelCache) get(
	ctx context.Context,
	fetch func(context.Context) ([]opencode.ModelInfo, error),
) ([]opencode.ModelInfo, error) {
	c.mu.Lock()

	// Lazy disk seed: first request after restart can avoid the CLI entirely.
	if c.fetchedAt.IsZero() {
		c.loadDiskLocked()
	}

	cached := c.models
	hasCache := !c.fetchedAt.IsZero() && len(c.models) > 0
	fresh := hasCache && time.Since(c.fetchedAt) < modelCacheTTL

	if fresh {
		c.mu.Unlock()
		return cloneModels(cached), nil
	}

	if hasCache {
		if !c.refreshing {
			c.refreshing = true
			go c.refresh(fetch)
		}
		c.mu.Unlock()
		return cloneModels(cached), nil
	}

	// Cold: join an in-flight fetch or start one.
	if c.refreshing {
		ch := make(chan coldResult, 1)
		c.waiters = append(c.waiters, ch)
		c.mu.Unlock()
		select {
		case res := <-ch:
			return res.models, res.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	c.refreshing = true
	c.mu.Unlock()

	models, err := fetch(ctx)
	c.finishCold(models, err)
	if err != nil {
		return nil, err
	}
	return cloneModels(models), nil
}

// finishCold stores a successful cold fetch, releases waiters, and persists.
func (c *modelCache) finishCold(models []opencode.ModelInfo, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refreshing = false
	if err == nil && len(models) > 0 {
		c.models = models
		c.fetchedAt = time.Now()
		c.persistLocked()
	}
	waiters := c.waiters
	c.waiters = nil
	out := cloneModels(c.models)
	for _, w := range waiters {
		if err != nil {
			w <- coldResult{err: err}
		} else {
			w <- coldResult{models: out}
		}
	}
}

func (c *modelCache) refresh(
	fetch func(context.Context) ([]opencode.ModelInfo, error),
) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	models, err := fetch(ctx)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.refreshing = false
	// Keep the previous cache on failure or empty result rather than wiping a
	// good list because one refresh hiccupped.
	if err != nil || len(models) == 0 {
		return
	}
	c.models = models
	c.fetchedAt = time.Now()
	c.persistLocked()
}

// loadDiskLocked seeds memory from disk if the file is present and not ancient.
// Caller must hold c.mu. No-op when memory is already populated.
func (c *modelCache) loadDiskLocked() {
	if !c.fetchedAt.IsZero() && len(c.models) > 0 {
		return
	}
	data, err := os.ReadFile(c.diskPath())
	if err != nil {
		return
	}
	var env modelDiskEnvelope
	if json.Unmarshal(data, &env) != nil || len(env.Models) == 0 {
		return
	}
	if env.FetchedAt.IsZero() {
		return
	}
	if time.Since(env.FetchedAt) > modelCacheDiskMaxAge {
		return
	}
	c.models = env.Models
	c.fetchedAt = env.FetchedAt
}

// persistLocked writes the current list to disk. Caller holds c.mu.
func (c *modelCache) persistLocked() {
	if len(c.models) == 0 || c.fetchedAt.IsZero() {
		return
	}
	path := c.diskPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	env := modelDiskEnvelope{FetchedAt: c.fetchedAt, Models: c.models}
	data, err := json.Marshal(env)
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// seedFromDisk loads disk into memory without fetching. Used by WarmModels so
// the first HTTP request never blocks even before the CLI refresh finishes.
func (c *modelCache) seedFromDisk() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loadDiskLocked()
}

// kickRefresh starts a background CLI refresh even when the in-memory list is
// still within TTL. Coalesces with an already-running refresh. Used when the
// user opens Settings so the catalog revalidates without blocking paint.
func (c *modelCache) kickRefresh(
	fetch func(context.Context) ([]opencode.ModelInfo, error),
) {
	c.mu.Lock()
	if c.refreshing {
		c.mu.Unlock()
		return
	}
	c.refreshing = true
	c.mu.Unlock()
	go c.refresh(fetch)
}

func cloneModels(in []opencode.ModelInfo) []opencode.ModelInfo {
	if len(in) == 0 {
		return nil
	}
	out := make([]opencode.ModelInfo, len(in))
	copy(out, in)
	return out
}
