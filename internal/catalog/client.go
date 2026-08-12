package catalog

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

//go:embed catalog.snapshot.json
var snapshotJSON []byte

const (
	defaultBaseURL = "https://models.dev"
	userAgent      = "modeltui/1.0 (+https://github.com/desenyon/ModelTUI)"
)

// Client fetches models.dev JSON endpoints.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	CacheDir   string
}

// NewClient returns a client with sensible defaults.
func NewClient() *Client {
	cache := filepath.Join(userCacheDir(), "modeltui")
	return &Client{
		BaseURL: defaultBaseURL,
		HTTPClient: &http.Client{
			Timeout: 45 * time.Second,
		},
		CacheDir: cache,
	}
}

func userCacheDir() string {
	if d, err := os.UserCacheDir(); err == nil && d != "" {
		return d
	}
	return os.TempDir()
}

// ParseCatalog decodes catalog JSON bytes.
func ParseCatalog(data []byte) (*Catalog, error) {
	var cat Catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return nil, err
	}
	if cat.Models == nil {
		cat.Models = map[string]CanonicalModel{}
	}
	if cat.Providers == nil {
		cat.Providers = map[string]Provider{}
	}
	for id, p := range cat.Providers {
		if p.ID == "" {
			p.ID = id
			cat.Providers[id] = p
		}
		if p.Models == nil {
			p.Models = map[string]OfferingModel{}
			cat.Providers[id] = p
		}
	}
	for id, m := range cat.Models {
		if m.ID == "" {
			m.ID = id
			cat.Models[id] = m
		}
	}
	return &cat, nil
}

func (c *Client) cachePath() string {
	return filepath.Join(c.CacheDir, "catalog.json")
}

func (c *Client) writeCache(cat *Catalog) error {
	if err := os.MkdirAll(c.CacheDir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(cat)
	if err != nil {
		return err
	}
	tmp := c.cachePath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.cachePath())
}

func (c *Client) readCache() (*Catalog, error) {
	data, err := os.ReadFile(c.cachePath())
	if err != nil {
		return nil, err
	}
	return ParseCatalog(data)
}

// Ensure CacheDir exists (used by tests / installers).
func (c *Client) EnsureCacheDir() error {
	return os.MkdirAll(c.CacheDir, 0o755)
}

// WarmFromSnapshot writes the embedded snapshot into the cache if missing.
func (c *Client) WarmFromSnapshot() error {
	if _, err := os.Stat(c.cachePath()); err == nil {
		return nil
	}
	cat, err := ParseCatalog(snapshotJSON)
	if err != nil {
		return err
	}
	return c.writeCache(cat)
}

// Ping is a lightweight connectivity check (HEAD) that still respects spacing.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, c.BaseURL+"/catalog.json", nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return fmt.Errorf("ping status %s", res.Status)
	}
	return nil
}
