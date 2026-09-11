package evals

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const costTolerance = 1e-9

func costNearlyEqual(a, b float64) bool {
	return math.Abs(a-b) <= costTolerance
}

// resetPricingState clears the in-process catalog state so each test starts
// from a cold cache.
func resetPricingState() {
	pricingMu.Lock()
	defer pricingMu.Unlock()
	pricingRows = nil
	pricingTriedFetch = false
}

// redirectPricing points the catalog fetcher and the cache path at a test
// sandbox; cleanup restores the production values.
func redirectPricing(t *testing.T, baseURL, cachePath string) {
	t.Helper()
	origURL, origPath := modelsDevBaseURL, pricingCachePath
	modelsDevBaseURL = baseURL
	pricingCachePath = func() string { return cachePath }
	resetPricingState()
	t.Cleanup(func() {
		modelsDevBaseURL = origURL
		pricingCachePath = origPath
		resetPricingState()
	})
}

// stubPricingServer starts a counting httptest server answering /api.json with
// body at status, and points pricing at it. Returns the request counter so
// tests can prove how many fetches happened.
func stubPricingServer(t *testing.T, body string, status int) *int {
	t.Helper()
	requests := new(int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*requests++
		if r.URL.Path != "/api.json" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	redirectPricing(t, srv.URL, filepath.Join(t.TempDir(), "models-dev-cache.json"))
	t.Cleanup(srv.Close)
	return requests
}

// deadPricing points pricing at a closed server — connects fail instantly, no
// live network — with an empty temp cache, forcing the embedded fallback table.
func deadPricing(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()
	redirectPricing(t, url, filepath.Join(t.TempDir(), "models-dev-cache.json"))
}

// seedPricingCache writes a cache file at path whose catalog went stale by age.
func seedPricingCache(t *testing.T, path string, fetchedAt time.Time, rows []modelPrice) {
	t.Helper()
	data, err := json.Marshal(pricingCacheFile{FetchedAtUnix: fetchedAt.Unix(), Rows: rows})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// stubCatalog is a minimal models.dev api.json exercising every shape the
// parser must tolerate: a priced model listed by two providers (the
// sorted-first one must win), a model without a cost object, and a provider
// without models. claude-sonnet-4-5 carries rates that differ from the
// embedded fallback row so tests can tell which table served.
func stubCatalog() string {
	return `{
		"aaa-vendor": {"models": {"test-model-alpha": {"cost": {"input": 2, "output": 8}}}},
		"zzz-gateway": {"models": {
			"test-model-alpha": {"cost": {"input": 99, "output": 99}},
			"test-model-free": {}
		}},
		"claude": {"models": {"claude-sonnet-4-5": {"cost": {"input": 7, "output": 70}}}},
		"empty-provider": {}
	}`
}

// Known models price from the embedded fallback table within float tolerance
// when the catalog is unreachable. Expected values are hardcoded so an
// accidental table edit turns the test red.
func TestLookupCostKnownModels(t *testing.T) {
	deadPricing(t)
	cases := []struct {
		name      string
		model     string
		tokensIn  int64
		tokensOut int64
		wantUSD   float64
		wantKnown bool
	}{
		{"claude sonnet 4.5 1M/1M", "claude-sonnet-4-5", 1_000_000, 1_000_000, 18.00, true},
		{"claude opus 4.1 partial", "claude-opus-4-1", 250_000, 100_000, 11.25, true},
		{"gpt-5-nano small totals", "gpt-5-nano", 500_000, 100_000, 0.065, true},
		{"gemini 2.5 flash", "gemini-2.5-flash", 2_000_000, 500_000, 1.85, true},
		{"zero tokens still known", "gpt-5", 0, 0, 0, true},
	}
	for _, tc := range cases {
		usd, known := LookupCost(tc.model, tc.tokensIn, tc.tokensOut)
		if known != tc.wantKnown || !costNearlyEqual(usd, tc.wantUSD) {
			t.Errorf("%s: LookupCost(%q,%d,%d) = (%v,%v), want (%v,%v)",
				tc.name, tc.model, tc.tokensIn, tc.tokensOut, usd, known, tc.wantUSD, tc.wantKnown)
		}
	}
}

func TestLookupCostUnknownModel(t *testing.T) {
	deadPricing(t)
	cases := []string{"totally-unknown-llm", "sonnet-x", "", "   "}
	for _, model := range cases {
		usd, known := LookupCost(model, 1_000_000, 1_000_000)
		if known || usd != 0 {
			t.Errorf("LookupCost(%q) = (%v,%v), want (0,false)", model, usd, known)
		}
	}
}

// The provider prefix must not affect the price; casing is normalized too.
func TestLookupCostStripsProviderPrefix(t *testing.T) {
	deadPricing(t)
	cases := []struct {
		prefixed string
		bare     string
		wantUSD  float64
	}{
		{"anthropic/claude-haiku-4-5", "claude-haiku-4-5", 6.00}, // 1M/1M at 1/5
		{"openai/gpt-5-mini", "gpt-5-mini", 2.25},                // 1M/1M at 0.25/2
		{"google/gemini-2.5-pro", "gemini-2.5-pro", 11.25},       // 1M/1M at 1.25/10
		{"Anthropic/Claude-Sonnet-4-5", "claude-sonnet-4-5", 18.00},
	}
	for _, tc := range cases {
		pUSD, pKnown := LookupCost(tc.prefixed, 1_000_000, 1_000_000)
		bUSD, bKnown := LookupCost(tc.bare, 1_000_000, 1_000_000)
		if !pKnown || !bKnown || !costNearlyEqual(pUSD, bUSD) {
			t.Errorf("prefix mismatch: (%q)=(%v,%v) vs (%q)=(%v,%v)",
				tc.prefixed, pUSD, pKnown, tc.bare, bUSD, bKnown)
		}
		if !costNearlyEqual(pUSD, tc.wantUSD) {
			t.Errorf("LookupCost(%q) = %v, want %v", tc.prefixed, pUSD, tc.wantUSD)
		}
	}
}

// Dated snapshots and dot variants match through the longest known substring.
func TestLookupCostSubstringFallback(t *testing.T) {
	deadPricing(t)
	cases := []struct {
		name    string
		model   string
		wantUSD float64 // price for 1M tokens in + 1M tokens out
	}{
		// Dated sonnet snapshot prices via "claude-sonnet-4" (3/15), not the family fallback.
		{"dated sonnet", "claude-sonnet-4-20250514", 18.00},
		// flash-lite-preview must hit the LONGEST match (0.10/0.40 = 0.50), not flash (2.80).
		{"flash lite preview", "gemini-2.5-flash-lite-preview-06-17", 0.50},
		// opus 4.5 snapshot must hit "claude-opus-4-5" (5/25 = 30), not "claude-opus" (90).
		{"dated opus 4.5", "claude-opus-4-5-20251101", 30.00},
		// Dot variant prices through "gpt-5" (1.25/10).
		{"gpt 5.1", "gpt-5.1", 11.25},
	}
	for _, tc := range cases {
		usd, known := LookupCost(tc.model, 1_000_000, 1_000_000)
		if !known || !costNearlyEqual(usd, tc.wantUSD) {
			t.Errorf("%s: LookupCost(%q) = (%v,%v), want (%v,true)", tc.name, tc.model, usd, known, tc.wantUSD)
		}
	}
}

// A fetched catalog is parsed and served: priced models cost at their catalog
// rates, catalog rates win over the embedded fallback for the same model id,
// cost-less models stay unknown, and duplicate ids resolve deterministically
// to the sorted-first provider.
func TestLookupCostFetchesAndParsesCatalog(t *testing.T) {
	if requests := stubPricingServer(t, stubCatalog(), http.StatusOK); *requests != 0 {
		t.Fatalf("stub counted %d requests before any lookup", *requests)
	}
	usd, known := LookupCost("test-model-alpha", 1_000_000, 1_000_000)
	if !known || !costNearlyEqual(usd, 10.00) { // aaa-vendor 2/8, not zzz-gateway 99/99
		t.Errorf("catalog model = (%v,%v), want (10,true)", usd, known)
	}
	usd, known = LookupCost("claude-sonnet-4-5", 1_000_000, 1_000_000)
	if !known || !costNearlyEqual(usd, 77.00) { // catalog 7/70, not the 3/15 fallback row
		t.Errorf("catalog beats fallback = (%v,%v), want (77,true)", usd, known)
	}
	if usd, known := LookupCost("test-model-free", 1_000_000, 1_000_000); known || usd != 0 {
		t.Errorf("cost-less model = (%v,%v), want (0,false)", usd, known)
	}
	if usd, known := LookupCost("totally-unknown-llm", 1_000_000, 1_000_000); known || usd != 0 {
		t.Errorf("unknown with catalog active = (%v,%v), want (0,false)", usd, known)
	}
}

// Within one process the catalog is fetched once: later lookups reuse the
// in-memory table without a second fetch.
func TestLookupCostCacheHitSkipsSecondFetch(t *testing.T) {
	requests := stubPricingServer(t, stubCatalog(), http.StatusOK)
	if _, known := LookupCost("test-model-alpha", 1_000_000, 1_000_000); !known {
		t.Fatal("first lookup should price from the fetched catalog")
	}
	if _, known := LookupCost("test-model-alpha", 1_000_000, 1_000_000); !known {
		t.Fatal("second lookup should price from the in-memory table")
	}
	if *requests != 1 {
		t.Errorf("stub served %d requests, want 1 (no second fetch)", *requests)
	}
}

// A fresh on-disk cache (written by an earlier run) serves with zero fetches.
func TestLookupCostFreshDiskCacheSkipsFetch(t *testing.T) {
	requests := stubPricingServer(t, stubCatalog(), http.StatusOK)
	seedPricingCache(t, pricingCachePath(), time.Now(),
		[]modelPrice{{Name: "cache-only-model", In: 3, Out: 9}})
	usd, known := LookupCost("cache-only-model", 1_000_000, 1_000_000)
	if !known || !costNearlyEqual(usd, 12.00) {
		t.Errorf("cached model = (%v,%v), want (12,true)", usd, known)
	}
	if *requests != 0 {
		t.Errorf("stub served %d requests, want 0 (fresh cache must not fetch)", *requests)
	}
}

// A stale cache refreshes synchronously from the catalog and the cache file is
// rewritten with the fresh rows.
func TestLookupCostStaleCacheRefreshes(t *testing.T) {
	requests := stubPricingServer(t, stubCatalog(), http.StatusOK)
	seedPricingCache(t, pricingCachePath(), time.Now().Add(-8*24*time.Hour),
		[]modelPrice{{Name: "stale-only-model", In: 1, Out: 1}})
	usd, known := LookupCost("test-model-alpha", 1_000_000, 1_000_000)
	if !known || !costNearlyEqual(usd, 10.00) {
		t.Errorf("stale cache did not refresh: = (%v,%v), want (10,true)", usd, known)
	}
	if *requests != 1 {
		t.Errorf("stub served %d requests, want 1 (one refresh)", *requests)
	}
	data, err := os.ReadFile(pricingCachePath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "test-model-alpha") {
		t.Errorf("cache file not rewritten with fresh rows: %s", data)
	}
}

// When the refresh fails against a stale cache, the stale rows are served and
// scoring never fails — with exactly one bounded fetch attempt.
func TestLookupCostServesStaleWhenRefreshFails(t *testing.T) {
	requests := stubPricingServer(t, "ignored", http.StatusInternalServerError)
	seedPricingCache(t, pricingCachePath(), time.Now().Add(-8*24*time.Hour),
		[]modelPrice{{Name: "stale-only-model", In: 3, Out: 9}})
	usd, known := LookupCost("stale-only-model", 1_000_000, 1_000_000)
	if !known || !costNearlyEqual(usd, 12.00) {
		t.Errorf("stale serve = (%v,%v), want (12,true)", usd, known)
	}
	if *requests != 1 {
		t.Errorf("stub served %d requests, want 1 (one bounded attempt)", *requests)
	}
}

// No cache and the fetch fails (dead endpoint): the embedded fallback table
// prices the run and scoring still succeeds.
func TestLookupCostFallsBackOnFetchError(t *testing.T) {
	deadPricing(t)
	usd, known := LookupCost("claude-sonnet-4-5", 1_000_000, 1_000_000)
	if !known || !costNearlyEqual(usd, 18.00) { // fallback row 3/15
		t.Errorf("fallback after fetch error = (%v,%v), want (18,true)", usd, known)
	}
}

// A malformed or empty catalog body degrades to the fallback table, never an
// error and never a wrong price.
func TestLookupCostFallsBackOnBadCatalogBody(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"malformed json", `{"providers": [broken`},
		{"empty object", `{}`},
		{"unexpected shape", `{"providers": ["a", "b"]}`},
	}
	for _, tc := range cases {
		stubPricingServer(t, tc.body, http.StatusOK)
		usd, known := LookupCost("claude-sonnet-4-5", 1_000_000, 1_000_000)
		if !known || !costNearlyEqual(usd, 18.00) {
			t.Errorf("%s: fallback = (%v,%v), want (18,true)", tc.name, usd, known)
		}
		resetPricingState()
	}
}
