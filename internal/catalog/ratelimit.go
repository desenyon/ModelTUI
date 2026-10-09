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
	"strings"
	"time"
)

const (
	MinRequestSpacing = 45 * time.Second
	AutoRefreshEvery  = 15 * time.Minute
	StaleAfter        = 12 * time.Hour
	MaxBackoff        = 30 * time.Minute
)

var (
	ErrRateLimited = errors.New("rate limited")
	ErrNotModified = errors.New("not modified without a validated cache")
	ErrOffline     = errors.New("offline mode: network refresh disabled")
)

type RefreshResult struct {
	Catalog     *Catalog
	Source      string
	NotModified bool
	RetryAfter  time.Duration
	Err         error
	Warning     error // Valid live data is still returned when cache persistence fails.
}

type rateState struct {
	LastRequest  time.Time `json:"last_request"`
	LastSuccess  time.Time `json:"last_success"`
	ETag         string    `json:"etag,omitempty"`
	BackoffUntil time.Time `json:"backoff_until,omitempty"`
	BaseURL      string    `json:"base_url,omitempty"`
	CacheHash    string    `json:"cache_hash,omitempty"`
}

func refreshWait(st rateState, now time.Time) time.Duration {
	wait := max(time.Duration(0), st.BackoffUntil.Sub(now))
	if !st.LastRequest.IsZero() {
		wait = max(wait, MinRequestSpacing-now.Sub(st.LastRequest))
	}
	return wait
}

// stateLocked keeps throttling reliable even if disk writes fail. The endpoint
// binding prevents custom clients from reusing another server's validator.
func (c *Client) stateLocked() rateState {
	if c.state == nil {
		st := c.readRateState()
		if st.BaseURL != c.BaseURL {
			if st.BaseURL != "" || c.BaseURL != defaultBaseURL {
				st = rateState{}
			}
			st.ETag, st.CacheHash = "", ""
		}
		c.state = &st
	}
	return *c.state
}

func (c *Client) saveStateLocked(st rateState) error {
	st.BaseURL = c.BaseURL
	c.state = &st
	return c.writeRateState(st)
}

// force is retained for compatibility; it never bypasses spacing or backoff.
func (c *Client) CanRefresh(force bool) (bool, time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Offline {
		return false, 0
	}
	wait := refreshWait(c.stateLocked(), time.Now())
	if c.inFlight {
		return false, max(wait, time.Second)
	}
	return wait == 0, wait
}

func (c *Client) ShouldAutoRefresh() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Offline || c.inFlight {
		return false
	}
	st := c.stateLocked()
	return st.LastSuccess.IsZero() || time.Since(st.LastSuccess) >= AutoRefreshEvery
}

func (c *Client) RefreshCatalog(ctx context.Context, force bool) RefreshResult {
	if err := ctx.Err(); err != nil {
		return RefreshResult{Err: err}
	}
	c.mu.Lock()
	if c.Offline {
		c.mu.Unlock()
		return RefreshResult{Err: ErrOffline}
	}
	st := c.stateLocked()
	wait := refreshWait(st, time.Now())
	if c.inFlight || wait > 0 {
		c.mu.Unlock()
		return RefreshResult{Err: ErrRateLimited, RetryAfter: max(wait, time.Second)}
	}
	c.inFlight = true
	st.LastRequest = time.Now()
	warning := c.saveStateLocked(st)
	c.mu.Unlock()
	defer func() { c.mu.Lock(); c.inFlight = false; c.mu.Unlock() }()

	cached, hash, cacheErr := c.readCacheWithHash()
	validator := ""
	if cacheErr == nil && st.CacheHash != "" && hash == st.CacheHash {
		validator = st.ETag
	}
	cat, etag, status, retryAfter, err := c.fetchCatalogConditional(ctx, validator)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		if status == http.StatusTooManyRequests {
			if retryAfter <= 0 {
				retryAfter = 2 * MinRequestSpacing
			}
			st.BackoffUntil = time.Now().Add(min(retryAfter, MaxBackoff))
			warning = errors.Join(warning, c.saveStateLocked(st))
			return RefreshResult{Err: fmt.Errorf("%w: %v", ErrRateLimited, err), RetryAfter: refreshWait(st, time.Now()), Warning: warning}
		}
		return RefreshResult{Err: err, Warning: warning}
	}
	if status == http.StatusNotModified {
		if cacheErr != nil || validator == "" {
			return RefreshResult{Err: ErrNotModified}
		}
		st.LastSuccess, st.BackoffUntil = time.Now(), time.Time{}
		warning = errors.Join(warning, c.saveStateLocked(st))
		return RefreshResult{Catalog: cached, Source: "models.dev (not modified)", NotModified: true, Warning: warning}
	}
	// Store the payload before its validator. A digest mismatch after a crash or
	// another process's write forces an unconditional request on the next run.
	st.ETag, st.CacheHash = "", ""
	if cacheErr = c.writeCache(cat); cacheErr == nil {
		data, _ := json.Marshal(cat)
		st.ETag, st.CacheHash = etag, digest(data)
	}
	st.LastSuccess, st.BackoffUntil = time.Now(), time.Time{}
	warning = errors.Join(warning, cacheErr, c.saveStateLocked(st))
	source := "live models.dev"
	if warning != nil {
		source += " (cache unavailable)"
	}
	return RefreshResult{Catalog: cat, Source: source, Warning: warning}
}

func (c *Client) LoadCatalog(ctx context.Context) (*Catalog, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	var res RefreshResult
	if !c.Offline {
		res = c.RefreshCatalog(ctx, false)
	}
	if res.Err == nil && res.Catalog != nil {
		return res.Catalog, res.Source, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	if cached, err := c.readCache(); err == nil {
		c.mu.Lock()
		st := c.stateLocked()
		c.mu.Unlock()
		src := "disk cache"
		if st.LastSuccess.IsZero() || time.Since(st.LastSuccess) > StaleAfter {
			src += " (stale)"
		}
		if c.Offline || (res.Err != nil && !errors.Is(res.Err, ErrRateLimited)) {
			src += " (offline)"
		}
		return cached, src, nil
	}
	cat, err := ParseCatalog(snapshotJSON)
	if err != nil {
		return nil, "", errors.Join(res.Err, err)
	}
	return cat, "embedded snapshot", nil
}

func (c *Client) fetchCatalogConditional(ctx context.Context, etag string) (*Catalog, string, int, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+"/catalog.json", nil)
	if err != nil {
		return nil, "", 0, 0, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, "", 0, 0, err
	}
	defer res.Body.Close()
	retryAfter := parseRetryAfter(res.Header.Get("Retry-After"))
	if res.StatusCode == http.StatusNotModified {
		return nil, etag, res.StatusCode, 0, nil
	}
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, "", res.StatusCode, retryAfter, fmt.Errorf("HTTP %s: %s", res.Status, strings.TrimSpace(string(body)))
	}
	data, err := readBounded(res.Body, maxCatalogBytes)
	if err != nil {
		return nil, "", res.StatusCode, 0, err
	}
	cat, err := ParseCatalog(data)
	return cat, res.Header.Get("ETag"), res.StatusCode, 0, err
}

func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if secs, err := strconv.ParseInt(v, 10, 64); err == nil {
		if secs <= 0 {
			return 0
		}
		if secs >= int64(MaxBackoff/time.Second) {
			return MaxBackoff
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		return min(MaxBackoff, max(time.Duration(0), time.Until(t)))
	}
	return 0
}

func (c *Client) rateStatePath() string { return filepath.Join(c.CacheDir, "rate.json") }
func (c *Client) readRateState() rateState {
	var st rateState
	f, err := os.Open(c.rateStatePath())
	if err != nil {
		return st
	}
	defer f.Close()
	if json.NewDecoder(io.LimitReader(f, 16384)).Decode(&st) != nil {
		return rateState{}
	}
	return st
}
func (c *Client) writeRateState(st rateState) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(c.rateStatePath(), data)
}

func (c *Client) NextAutoRefreshIn() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	st := c.stateLocked()
	wait := refreshWait(st, time.Now())
	if !st.LastSuccess.IsZero() {
		wait = max(wait, AutoRefreshEvery-time.Since(st.LastSuccess))
	}
	return max(time.Second, wait)
}
