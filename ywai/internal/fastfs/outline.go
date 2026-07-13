package fastfs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// DefaultMaxSliceLines caps read_slice without an explicit limit.
const DefaultMaxSliceLines = 200

// OutlineResult is a summarized file view (not a full dump).
type OutlineResult struct {
	Path       string   `json:"path"`
	Lines      int      `json:"lines"`
	Bytes      int      `json:"bytes"`
	Language   string   `json:"language,omitempty"`
	Signatures []string `json:"signatures"`
	Sample     string   `json:"sample"`
	Note       string   `json:"note"`
}

// SliceResult is a bounded line range read.
type SliceResult struct {
	Path      string `json:"path"`
	Start     int    `json:"start"` // 1-based inclusive
	End       int    `json:"end"`   // 1-based inclusive
	Lines     int    `json:"totalLines"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

var (
	reGoSig = regexp.MustCompile(`^(func |type |const |var |package )`)
	reTSSig = regexp.MustCompile(`^(export )?(async )?(function |class |interface |type |const |enum )`)
	rePySig = regexp.MustCompile(`^(def |class |async def )`)
	reRSSig = regexp.MustCompile(`^(pub )?(fn |struct |enum |trait |impl |mod |const |static )`)
)

// ReadOutline returns structural signatures + elided sample via the file cache.
func (s *Service) ReadOutline(relOrAbs string) (*OutlineResult, error) {
	abs, err := s.root.Resolve(relOrAbs)
	if err != nil {
		return nil, err
	}
	data, err := s.cache.Get(abs)
	if err != nil {
		return nil, err
	}
	if isBinary(data) {
		return nil, fmt.Errorf("binary file: %s", s.root.Rel(abs))
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("non-utf8 file: %s", s.root.Rel(abs))
	}
	lines := strings.Split(string(data), "\n")
	lang := langFromExt(filepath.Ext(abs))
	var sigs []string
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if isSignature(lang, trim) {
			sigs = append(sigs, fmt.Sprintf("%d: %s", i+1, trim))
			if len(sigs) >= 40 {
				break
			}
		}
	}
	return &OutlineResult{
		Path:       s.root.Rel(abs),
		Lines:      len(lines),
		Bytes:      len(data),
		Language:   lang,
		Signatures: sigs,
		Sample:     elideSample(lines, 25, 10),
		Note:       "Prefer this outline over a full file dump. Use fastfs_read_slice for specific line ranges. For structural navigation prefer codegraph_explore when available.",
	}, nil
}

// ReadSlice returns lines [start, end] (1-based inclusive). maxLines caps the window.
func (s *Service) ReadSlice(relOrAbs string, start, end, maxLines int) (*SliceResult, error) {
	if maxLines <= 0 {
		maxLines = DefaultMaxSliceLines
	}
	abs, err := s.root.Resolve(relOrAbs)
	if err != nil {
		return nil, err
	}
	data, err := s.cache.Get(abs)
	if err != nil {
		return nil, err
	}
	if isBinary(data) {
		return nil, fmt.Errorf("binary file: %s", s.root.Rel(abs))
	}
	lines := strings.Split(string(data), "\n")
	total := len(lines)
	if start <= 0 {
		start = 1
	}
	if end <= 0 || end > total {
		end = total
	}
	if start > total {
		return &SliceResult{Path: s.root.Rel(abs), Start: start, End: start, Lines: total}, nil
	}
	if end < start {
		end = start
	}
	truncated := false
	if end-start+1 > maxLines {
		end = start + maxLines - 1
		truncated = true
	}
	return &SliceResult{
		Path:      s.root.Rel(abs),
		Start:     start,
		End:       end,
		Lines:     total,
		Content:   strings.Join(lines[start-1:end], "\n"),
		Truncated: truncated,
	}, nil
}

// Stat returns basic file metadata plus cache stats.
func (s *Service) Stat(relOrAbs string) (map[string]any, error) {
	abs, err := s.root.Resolve(relOrAbs)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	hits, misses, entries, bytes := s.cache.Stats()
	return map[string]any{
		"path":         s.root.Rel(abs),
		"size":         info.Size(),
		"modTime":      info.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
		"isDir":        info.IsDir(),
		"cacheHits":    hits,
		"cacheMisses":  misses,
		"cacheEntries": entries,
		"cacheBytes":   bytes,
	}, nil
}

func langFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".go":
		return "go"
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return "typescript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".md":
		return "markdown"
	default:
		return strings.TrimPrefix(strings.ToLower(ext), ".")
	}
}

func isSignature(lang, trim string) bool {
	switch lang {
	case "go":
		return reGoSig.MatchString(trim)
	case "typescript":
		return reTSSig.MatchString(trim)
	case "python":
		return rePySig.MatchString(trim)
	case "rust":
		return reRSSig.MatchString(trim)
	default:
		return reTSSig.MatchString(trim) || reGoSig.MatchString(trim) || rePySig.MatchString(trim)
	}
}

func elideSample(lines []string, head, tail int) string {
	n := len(lines)
	if n == 0 {
		return ""
	}
	if n <= head+tail+5 {
		return strings.Join(lines, "\n")
	}
	var b strings.Builder
	for i := 0; i < head && i < n; i++ {
		b.WriteString(lines[i])
		b.WriteByte('\n')
	}
	omitted := n - head - tail
	if omitted < 0 {
		omitted = 0
	}
	fmt.Fprintf(&b, "… %d lines omitted …\n", omitted)
	for i := n - tail; i < n; i++ {
		if i < 0 {
			continue
		}
		b.WriteString(lines[i])
		if i < n-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
