package fastfs

// Service is the in-process fast filesystem API (shared cache + ignore).
type Service struct {
	root   *Root
	cache  *FileCache
	ignore *IgnoreMatcher
}

// NewService creates a Service rooted at workspace (empty = cwd).
func NewService(workspace string) (*Service, error) {
	root, err := NewRoot(workspace)
	if err != nil {
		return nil, err
	}
	return &Service{
		root:   root,
		cache:  NewFileCache(DefaultMaxCacheBytes),
		ignore: LoadIgnore(root.Abs),
	}, nil
}

// RootAbs returns the absolute workspace path.
func (s *Service) RootAbs() string { return s.root.Abs }

// CacheStats exposes hit/miss for doctor/tests.
func (s *Service) CacheStats() (hits, misses int64, entries int, bytes int64) {
	return s.cache.Stats()
}
