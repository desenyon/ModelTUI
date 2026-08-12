package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// Rate-limit / freshness policy for models.dev.
// The catalog is a public static-ish JSON blob; we still space requests
// politely and honor 429 + Retry-After.
const (
	MinRequestSpacing = 45 * time.Second  // hard floor between network hits
	AutoRefreshEvery  = 15 * time.Minute  // background refresh cadence
	StaleAfter        = 12 * time.Hour    // prefer live data over older cache
	MaxBackoff        = 30 * time.Minute
)

var (
	ErrRateLimited = errors.New("rate limited")
	ErrNotModified = errors.New("not modified")
)

// RefreshResult is returned by a polite refresh attempt.
type RefreshResult struct {
	Catalog    *Catalog
	Source     string
	NotModified bool
	RetryAfter  time.Duration
	Err         error
}

type rateState struct {
	LastRequest time.Time `json:"last_request"`
	LastSuccess time.Time `json:"last_success"`
	ETag        string    `json:"etag,omitempty"`
	BackoffUntil time.Time `json:"backoff_until,omitempty"`
}

var refreshMu sync.Mutex

// CanRefresh reports whether a network refresh is currently allowed.
func (c *Client) CanRefresh(force bool) (ok bool, wait time.Duration) {
	st := c.readRateState()
	now := time.Now()
	if !st.BackoffUntil.IsZero() && now.Before(st.BackoffUntil) {
		return false, st.BackoffUntil.Sub(now)
	}
	if !force && !st.LastRequest.IsZero() {
		elapsed := now.Sub(st.LastRequest)
		if elapsed < MinRequestSpacing {
			return false, MinRequestSpacing - elapsed
		}
	}
	if force && !st.LastRequest.IsZero() {
		elapsed := now.Sub(st.LastRequest)
		if elapsed < MinRequestSpacing {
			return false, MinRequestSpacing - elapsed
		}
	}
	return true, 0
}

// ShouldAutoRefresh reports whether background refresh is due.
func (c *Client) ShouldAutoRefresh() bool {
	st := c.readRateState()
	if st.LastSuccess.IsZero() {
		return true
	}
	return time.Since(st.LastSuccess) >= AutoRefreshEvery
}

// RefreshCatalog performs a rate-limited conditional GET of catalog.json.
// force still respects MinRequestSpacing and active backoff.
func (c *Client) RefreshCatalog(ctx context.Context, force bool) RefreshResult {
	refreshMu.Lock()
	defer refreshMu.Unlock()

	if ok, wait := c.CanRefresh(force); !ok {
		return RefreshResult{
			Err:        fmt.Errorf("%w: retry in %s", ErrRateLimited, wait.Round(time.Second)),
			RetryAfter: wait,
		}
	}

	st := c.readRateState()
	st.LastRequest = time.Now()
	_ = c.writeRateState(st)

	cat, etag, status, retryAfter, err := c.fetchCatalogConditional(ctx, st.ETag)
	if err != nil {
		if status == http.StatusTooManyRequests {
			wait := retryAfter
			if wait <= 0 {
				wait = MinRequestSpacing * 2
			}
			if wait > MaxBackoff {
				wait = MaxBackoff
			}
			st.BackoffUntil = time.Now().Add(wait)
			_ = c.writeRateState(st)
			return RefreshResult{Err: fmt.Errorf("%w: %v", ErrRateLimited, err), RetryAfter: wait}
		}
		return RefreshResult{Err: err, RetryAfter: MinRequestSpacing}
	}

	if status == http.StatusNotModified {
		st.LastSuccess = time.Now()
		st.BackoffUntil = time.Time{}
		_ = c.writeRateState(st)
		if cached, cerr := c.readCache(); cerr == nil {
			return RefreshResult{Catalog: cached, Source: "models.dev (not modified)", NotModified: true}
		}
		return RefreshResult{NotModified: true, Err: ErrNotModified}
	}

	if etag != "" {
		st.ETag = etag
	}
	st.LastSuccess = time.Now()
	st.BackoffUntil = time.Time{}
	_ = c.writeRateState(st)
	_ = c.writeCache(cat)
	return RefreshResult{Catalog: cat, Source: "live models.dev"}
}

// LoadCatalog prefers a fresh live refresh when allowed, else cache/snapshot.
func (c *Client) LoadCatalog(ctx context.Context) (*Catalog, string, error) {
	res := c.RefreshCatalog(ctx, false)
	if res.Err == nil && res.Catalog != nil {
		return res.Catalog, res.Source, nil
	}

	if cached, err := c.readCache(); err == nil {
		st := c.readRateState()
		src := "disk cache"
		if !st.LastSuccess.IsZero() && time.Since(st.LastSuccess) > StaleAfter {
			src = "disk cache (stale)"
		}
		if res.Err != nil && !errors.Is(res.Err, ErrRateLimited) && !errors.Is(res.Err, ErrNotModified) {
			src = "disk cache (offline)"
		}
		return cached, src, nil
	}

	cat, err := ParseCatalog(snapshotJSON)
	if err != nil {
		if res.Err != nil {
			return nil, "", fmt.Errorf("fetch catalog: %w (snapshot: %v)", res.Err, err)
		}
		return nil, "", err
	}
	return cat, "embedded snapshot", nil
}

func (c *Client) fetchCatalogConditional(ctx context.Context, etag string) (*Catalog, string, int, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/catalog.json", nil)
	if err != nil {
		return nil, "", 0, 0, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, "", 0, 0, err
	}
	defer res.Body.Close()

	retryAfter := parseRetryAfter(res.Header.Get("Retry-After"))
	newETag := res.Header.Get("ETag")

	if res.StatusCode == http.StatusNotModified {
		return nil, etag, res.StatusCode, 0, nil
	}
	if res.StatusCode == http.StatusTooManyRequests {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 256))
		return nil, "", res.StatusCode, retryAfter, fmt.Errorf("HTTP 429: %s", string(body))
	}
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, "", res.StatusCode, retryAfter, fmt.Errorf("unexpected status %s: %s", res.Status, string(body))
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, "", res.StatusCode, 0, err
	}
	cat, err := ParseCatalog(data)
	if err != nil {
		return nil, "", res.StatusCode, 0, err
	}
	return cat, newETag, res.StatusCode, 0, nil
}

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}

func (c *Client) rateStatePath() string {
	return filepath.Join(c.CacheDir, "rate.json")
}

func (c *Client) readRateState() rateState {
	data, err := os.ReadFile(c.rateStatePath())
	if err != nil {
		return rateState{}
	}
	var st rateState
	if json.Unmarshal(data, &st) != nil {
		return rateState{}
	}
	return st
}

func (c *Client) writeRateState(st rateState) error {
	if err := os.MkdirAll(c.CacheDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := c.rateStatePath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.rateStatePath())
}

// NextAutoRefreshIn returns delay until the next background refresh attempt.
func (c *Client) NextAutoRefreshIn() time.Duration {
	st := c.readRateState()
	now := time.Now()
	candidates := []time.Duration{AutoRefreshEvery}
	if !st.LastSuccess.IsZero() {
		remaining := AutoRefreshEvery - now.Sub(st.LastSuccess)
		if remaining > 0 {
			candidates[0] = remaining
		} else {
			candidates[0] = 0
		}
	}
	if !st.BackoffUntil.IsZero() && now.Before(st.BackoffUntil) {
		candidates = append(candidates, st.BackoffUntil.Sub(now))
	}
	if !st.LastRequest.IsZero() {
		remaining := MinRequestSpacing - now.Sub(st.LastRequest)
		if remaining > 0 {
			candidates = append(candidates, remaining)
		}
	}
	wait := candidates[0]
	for _, d := range candidates[1:] {
		if d > wait {
			wait = d
		}
	}
	if wait < time.Second {
		wait = time.Second
	}
	return wait
}
