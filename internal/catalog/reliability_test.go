package catalog

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const validCatalog = `{"models":{"lab/m":{"name":"Model"}},"providers":{}}`

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient()
	c.BaseURL, c.CacheDir = srv.URL, t.TempDir()
	return c
}

func TestRejectNonCatalogPayloads(t *testing.T) {
	for _, body := range []string{`null`, `{}`, `{"error":"unavailable"}`, `{"models":null,"providers":{}}`, `{"models":{},"providers":null}`} {
		t.Run(body, func(t *testing.T) {
			if _, err := ParseCatalog([]byte(body)); err == nil {
				t.Fatal("accepted non-catalog payload")
			}
		})
	}
}

func TestMissingCacheDoesNotSendValidator(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		t.Run(fmt.Sprint(corrupt), func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("If-None-Match"); got != "" {
					t.Errorf("sent validator without usable cache: %s", got)
					w.WriteHeader(304)
					return
				}
				fmt.Fprint(w, validCatalog)
			})
			if err := c.writeRateState(rateState{ETag: `"old"`}); err != nil {
				t.Fatal(err)
			}
			if corrupt {
				if err := os.WriteFile(c.cachePath(), []byte("broken"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			res := c.RefreshCatalog(t.Context(), false)
			if res.Err != nil || res.Catalog == nil {
				t.Fatalf("did not recover catalog: %+v", res)
			}
		})
	}
}

func TestThrottleSurvivesUnwritableCache(t *testing.T) {
	var hits atomic.Int32
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); fmt.Fprint(w, validCatalog) })
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	c.CacheDir = file
	if res := c.RefreshCatalog(t.Context(), true); res.Err != nil {
		t.Fatal(res.Err)
	}
	if res := c.RefreshCatalog(t.Context(), true); !errors.Is(res.Err, ErrRateLimited) {
		t.Fatalf("expected throttling after persistence failure, got %v", res.Err)
	}
	if hits.Load() != 1 {
		t.Fatalf("sent %d requests", hits.Load())
	}
}

func TestNotModifiedRequiresCacheAndRetainsCatalog(t *testing.T) {
	var hits int
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits == 1 {
			w.Header().Set("ETag", `"first"`)
			fmt.Fprint(w, validCatalog)
			return
		}
		if r.Header.Get("If-None-Match") != `"first"` {
			t.Errorf("missing validator")
		}
		w.WriteHeader(http.StatusNotModified)
	})
	if res := c.RefreshCatalog(t.Context(), true); res.Err != nil {
		t.Fatal(res.Err)
	}
	// Simulate the next invocation after request spacing expires.
	st := c.readRateState()
	st.LastRequest = time.Now().Add(-time.Minute)
	if err := c.writeRateState(st); err != nil {
		t.Fatal(err)
	}
	next := NewClient()
	next.BaseURL, next.CacheDir = c.BaseURL, c.CacheDir
	res := next.RefreshCatalog(t.Context(), false)
	if res.Err != nil || !res.NotModified || res.Catalog == nil || len(res.Catalog.Models) != 1 {
		t.Fatalf("bad 304 result: %+v", res)
	}
}

func TestSuccessfulResponseWithoutETagClearsOldValidator(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, validCatalog) })
	if err := c.writeRateState(rateState{ETag: `"old"`}); err != nil {
		t.Fatal(err)
	}
	if res := c.RefreshCatalog(t.Context(), false); res.Err != nil {
		t.Fatal(res.Err)
	}
	if got := c.readRateState().ETag; got != "" {
		t.Fatalf("retained obsolete ETag: %q", got)
	}
}

func TestRejectOversizedCatalog(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, strings.Repeat(" ", 32<<20)+validCatalog) })
	if res := c.RefreshCatalog(t.Context(), false); res.Err == nil {
		t.Fatal("accepted oversized payload")
	}
}

func TestRetryAfterOverflowIsCapped(t *testing.T) {
	if got := parseRetryAfter("9223372036854775807"); got != MaxBackoff {
		t.Fatalf("overflowed Retry-After: %s", got)
	}
}

func TestCanceledLoadDoesNotFallBack(t *testing.T) {
	c := NewClient()
	c.CacheDir = t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := c.LoadCatalog(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation hidden by snapshot: %v", err)
	}
}

func TestConcurrentRefreshSendsOneRequest(t *testing.T) {
	var hits atomic.Int32
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); fmt.Fprint(w, validCatalog) })
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() { c.RefreshCatalog(t.Context(), false) })
	}
	wg.Wait()
	if hits.Load() != 1 {
		t.Fatalf("sent %d concurrent requests", hits.Load())
	}
}

func TestInvalidRefreshPreservesExistingCache(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{"error":"unavailable"}`) })
	cat, err := ParseCatalog([]byte(validCatalog))
	if err != nil {
		t.Fatal(err)
	}
	if err = c.writeCache(cat); err != nil {
		t.Fatal(err)
	}
	if res := c.RefreshCatalog(t.Context(), false); res.Err == nil {
		t.Fatal("accepted invalid refresh")
	}
	got, err := c.readCache()
	if err != nil || len(got.Models) != 1 {
		t.Fatalf("cache lost: %+v %v", got, err)
	}
}

func TestChangedCacheInvalidatesETag(t *testing.T) {
	var validator string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		validator = r.Header.Get("If-None-Match")
		fmt.Fprint(w, validCatalog)
	})
	old, _ := ParseCatalog([]byte(validCatalog))
	c.writeCache(old)
	_, hash, _ := c.readCacheWithHash()
	c.writeRateState(rateState{BaseURL: c.BaseURL, CacheHash: hash, ETag: `"old"`})
	os.WriteFile(c.cachePath(), []byte(`{"models":{},"providers":{}}`), 0600)
	if res := c.RefreshCatalog(t.Context(), false); res.Err != nil {
		t.Fatal(res.Err)
	}
	if validator != "" {
		t.Fatalf("sent validator for changed cache: %s", validator)
	}
}
