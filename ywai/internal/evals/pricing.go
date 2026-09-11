package evals

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// modelPrice is one row of a pricing table: input and output cost in USD
// per 1M tokens for the model named in Name.
type modelPrice struct {
	Name string
	In   float64
	Out  float64
}

// fallbackPricing is the embedded, code-adjacent table used ONLY when the live
// models.dev catalog is unavailable: offline run, fetch error, malformed body,
// or no cache to serve stale. Rates are frozen as of early 2026 and drift, so
// treat them as approximations, not quotes.
//
// Matching is exact first, then longest known substring, so dated snapshots such
// as "claude-sonnet-4-20250514" resolve through the "claude-sonnet-4" row. Rows
// are ordered narrowest-match-last per family only for readability; selection is
// by longest name, not order.
var fallbackPricing = []modelPrice{
	// Anthropic Claude (rates as of early 2026).
	{"claude-opus-4-5", 5.00, 25.00},
	{"claude-opus-4-1", 15.00, 75.00},
	{"claude-opus-4", 15.00, 75.00},
	{"claude-opus", 15.00, 75.00}, // generic fallback: claude-3-opus and older opus tiers
	{"claude-sonnet-4-5", 3.00, 15.00},
	{"claude-sonnet-4", 3.00, 15.00},
	{"claude-sonnet", 3.00, 15.00}, // generic fallback: 3.5/3.7 sonnet share the rate
	{"claude-haiku-4-5", 1.00, 5.00},
	{"claude-3-5-haiku", 0.80, 4.00},
	{"claude-haiku", 1.00, 5.00}, // generic fallback: current-generation haiku rate

	// OpenAI GPT-5 family. "gpt-5.1" and "-codex" variants price through "gpt-5".
	{"gpt-5-pro", 15.00, 120.00},
	{"gpt-5-mini", 0.25, 2.00},
	{"gpt-5-nano", 0.05, 0.40},
	{"gpt-5", 1.25, 10.00},

	// Google Gemini 2.5 family. Pro prices are the <=200k-prompt tier (above it: 2.50/15).
	{"gemini-2.5-flash-lite", 0.10, 0.40},
	{"gemini-2.5-flash", 0.30, 2.50},
	{"gemini-2.5-pro", 1.25, 10.00},
}

const (
	// pricingCacheTTL is how long a fetched catalog stays trusted on disk.
	pricingCacheTTL = 7 * 24 * time.Hour
	// pricingFetchTimeout bounds the synchronous refresh so a slow network can
	// not stall scoring.
	pricingFetchTimeout = 2 * time.Second
)

// modelsDevBaseURL serves the model cost catalog; "/api.json" is appended. A
// var so tests run against an httptest stub — unit tests never touch the live
// network.
var modelsDevBaseURL = "https://models.dev"

// pricingCachePath is where the fetched catalog is cached on disk, under
// config.DataDir(). A var so tests redirect it to a temp directory.
var pricingCachePath = func() string {
	return filepath.Join(config.DataDir(), "models-dev-cache.json")
}

var pricingHTTPClient = &http.Client{Timeout: pricingFetchTimeout}

// models.dev api.json shape (verified against the live endpoint): an object
// keyed by provider id, each provider holding a models object keyed by model
// id; priced models carry a cost object whose input/output are USD per 1M
// tokens — the same unit modelPrice uses. Anything the catalog omits (a model
// without cost, a provider without models) simply contributes no rows.
type modelsDevProvider struct {
	Models map[string]modelsDevModel `json:"models"`
}

type modelsDevModel struct {
	Cost *modelsDevCost `json:"cost"`
}

type modelsDevCost struct {
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
}

// pricingCacheFile is the on-disk format of the fetched catalog.
type pricingCacheFile struct {
	FetchedAtUnix int64        `json:"fetched_at_unix"`
	Rows          []modelPrice `json:"rows"`
}

var (
	pricingMu         sync.Mutex
	pricingRows       []modelPrice // catalog rows served this process; nil until loaded
	pricingTriedFetch bool         // the one bounded refresh attempt already happened
)

// LookupCost prices one attempt in US dollars from the model name and the token
// totals read back from OpenCode's database. The name is normalized by stripping
// a provider/ prefix ("anthropic/claude-sonnet-4-5" -> "claude-sonnet-4-5"),
// then matched exactly against the active price table, then by longest known
// substring so dated snapshots still price. The active table is the models.dev
// catalog — served from a fresh disk cache when possible, otherwise fetched —
// and fallbackPricing takes over only when the catalog is unavailable. known is
// false (and usd 0) when no row matches — the caller decides how to render an
// unknown price.
func LookupCost(model string, tokensIn, tokensOut int64) (usd float64, known bool) {
	m := strings.ToLower(strings.TrimSpace(model))
	if i := strings.IndexByte(m, '/'); i >= 0 {
		m = m[i+1:]
	}
	if m == "" {
		return 0, false
	}
	rows := getPricing()
	if rows == nil {
		rows = fallbackPricing
	}
	var best *modelPrice
	for i := range rows {
		p := &rows[i]
		if p.Name == m {
			best = p
			break
		}
		if strings.Contains(m, p.Name) && (best == nil || len(p.Name) > len(best.Name)) {
			best = p
		}
	}
	if best == nil {
		return 0, false
	}
	return (best.In*float64(tokensIn) + best.Out*float64(tokensOut)) / 1e6, true
}

// getPricing returns the price table to match against: the models.dev catalog
// from the fresh disk cache or a just-completed fetch, the stale cache when a
// refresh fails, or nil when nothing is available (the caller falls back). It
// makes at most one bounded network attempt per process — the first lookup
// after the cache goes stale — so an offline scoring run pays the fetch timeout
// once, not per attempt. A failed refresh is retried by the next scoring run;
// scoring never fails and never blocks on the network.
func getPricing() []modelPrice {
	pricingMu.Lock()
	defer pricingMu.Unlock()
	if pricingTriedFetch {
		return pricingRows
	}
	pricingTriedFetch = true

	rows, fetchedAt := readPricingCache()
	if !fetchedAt.IsZero() && time.Since(fetchedAt) < pricingCacheTTL {
		pricingRows = rows
		return pricingRows
	}
	if fresh, err := fetchPricing(); err == nil {
		pricingRows = fresh
		writePricingCache(fresh)
		return pricingRows
	}
	// Refresh failed (offline, bad response, unexpected shape): serve whatever
	// the cache still holds, even stale — stale rates beat a frozen table.
	pricingRows = rows
	return pricingRows
}

// fetchPricing downloads and parses the models.dev catalog into rows. Any
// transport, status, or shape problem is an error; the caller degrades instead.
func fetchPricing() ([]modelPrice, error) {
	resp, err := pricingHTTPClient.Get(modelsDevBaseURL + "/api.json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models.dev: %s", resp.Status)
	}
	var providers map[string]modelsDevProvider
	if err := json.NewDecoder(resp.Body).Decode(&providers); err != nil {
		return nil, err
	}
	rows := catalogRows(providers)
	if len(rows) == 0 {
		return nil, errors.New("models.dev: no priced models in response")
	}
	return rows, nil
}

// catalogRows flattens the provider->models catalog into name-keyed rows.
// Providers are visited in sorted order and the first listing of a model id
// wins, so the result is deterministic even where gateways re-list vendor
// models at their own rates. Rows come out sorted by name for a stable cache.
func catalogRows(providers map[string]modelsDevProvider) []modelPrice {
	byName := make(map[string]modelPrice)
	for _, prov := range slices.Sorted(maps.Keys(providers)) {
		for id, m := range providers[prov].Models {
			if m.Cost == nil {
				continue
			}
			if _, seen := byName[id]; !seen {
				byName[id] = modelPrice{Name: id, In: m.Cost.Input, Out: m.Cost.Output}
			}
		}
	}
	rows := make([]modelPrice, 0, len(byName))
	for _, id := range slices.Sorted(maps.Keys(byName)) {
		rows = append(rows, byName[id])
	}
	return rows
}

// readPricingCache loads the disk cache, or reports (nil, zero time) when it
// is missing, malformed, or empty — a broken cache degrades to fallback.
func readPricingCache() ([]modelPrice, time.Time) {
	data, err := os.ReadFile(pricingCachePath())
	if err != nil {
		return nil, time.Time{}
	}
	var c pricingCacheFile
	if err := json.Unmarshal(data, &c); err != nil || len(c.Rows) == 0 || c.FetchedAtUnix <= 0 {
		return nil, time.Time{}
	}
	return c.Rows, time.Unix(c.FetchedAtUnix, 0)
}

// writePricingCache persists the fetched catalog for the next run. Best-effort:
// an unwritable home leaves scoring on the fallback table, so errors are ignored.
func writePricingCache(rows []modelPrice) {
	data, err := json.Marshal(pricingCacheFile{FetchedAtUnix: time.Now().Unix(), Rows: rows})
	if err != nil {
		return
	}
	path := pricingCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path) // os.Rename replaces an existing file on every platform
}
