package fastfs

import (
	"container/list"
	"os"
	"sync"
	"time"
)

// DefaultMaxCacheBytes is the soft cap for cached file contents (~64 MiB).
const DefaultMaxCacheBytes int64 = 64 << 20

type cacheEntry struct {
	path    string
	modTime time.Time
	size    int64
	content []byte
	element *list.Element // for LRU
}

// FileCache is an mtime-keyed content cache shared by find/search/read.
// Safe for concurrent use. Soft maxBytes with LRU eviction.
type FileCache struct {
	mu       sync.Mutex
	entries  map[string]*cacheEntry
	order    *list.List // front = most recently used
	curBytes int64
	maxBytes int64
	hits     int64
	misses   int64
}

// NewFileCache creates a cache with the given soft size cap (bytes).
// maxBytes <= 0 uses DefaultMaxCacheBytes.
func NewFileCache(maxBytes int64) *FileCache {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxCacheBytes
	}
	return &FileCache{
		entries:  make(map[string]*cacheEntry),
		order:    list.New(),
		maxBytes: maxBytes,
	}
}

// Stats returns hit/miss counters (for tests and doctor).
func (c *FileCache) Stats() (hits, misses int64, entries int, bytes int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits, c.misses, len(c.entries), c.curBytes
}

// Get returns file bytes if the cached mtime/size still match disk.
// On miss it reads from disk, stores, and returns the content.
func (c *FileCache) Get(absPath string) ([]byte, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, os.ErrInvalid
	}
	mod := info.ModTime()
	size := info.Size()

	c.mu.Lock()
	if e, ok := c.entries[absPath]; ok && e.modTime.Equal(mod) && e.size == size {
		c.hits++
		c.order.MoveToFront(e.element)
		out := e.content
		c.mu.Unlock()
		return out, nil
	}
	c.misses++
	c.mu.Unlock()

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	// Another goroutine may have filled it; re-check.
	if e, ok := c.entries[absPath]; ok && e.modTime.Equal(mod) && e.size == size {
		c.order.MoveToFront(e.element)
		return e.content, nil
	}
	c.putLocked(absPath, mod, size, data)
	return data, nil
}

// Invalidate removes a path from the cache (tests / explicit refresh).
func (c *FileCache) Invalidate(absPath string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeLocked(absPath)
}

func (c *FileCache) putLocked(path string, mod time.Time, size int64, data []byte) {
	if e, ok := c.entries[path]; ok {
		c.curBytes -= int64(len(e.content))
		c.order.Remove(e.element)
		delete(c.entries, path)
	}
	// Skip caching huge single files that exceed the whole budget.
	if int64(len(data)) > c.maxBytes {
		return
	}
	for c.curBytes+int64(len(data)) > c.maxBytes && c.order.Len() > 0 {
		oldest := c.order.Back()
		if oldest == nil {
			break
		}
		oldPath := oldest.Value.(string)
		c.removeLocked(oldPath)
	}
	e := &cacheEntry{
		path:    path,
		modTime: mod,
		size:    size,
		content: data,
	}
	e.element = c.order.PushFront(path)
	c.entries[path] = e
	c.curBytes += int64(len(data))
}

func (c *FileCache) removeLocked(path string) {
	e, ok := c.entries[path]
	if !ok {
		return
	}
	c.curBytes -= int64(len(e.content))
	c.order.Remove(e.element)
	delete(c.entries, path)
}
