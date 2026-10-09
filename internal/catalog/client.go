package catalog

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

//go:embed catalog.snapshot.json
var snapshotJSON []byte

const (
	defaultBaseURL  = "https://models.dev"
	userAgent       = "modeltui/1.0 (+https://github.com/desenyon/ModelTUI)"
	maxCatalogBytes = 32 << 20
)

// Client fetches models.dev data. Configure fields before use; share a pointer
// across callers. A client may be used concurrently but must not be copied.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	CacheDir   string
	Offline    bool
	mu         sync.Mutex
	state      *rateState
	inFlight   bool
}

func NewClient() *Client {
	return &Client{BaseURL: defaultBaseURL, HTTPClient: &http.Client{Timeout: 45 * time.Second}, CacheDir: filepath.Join(userCacheDir(), "modeltui")}
}

func userCacheDir() string {
	if d, err := os.UserCacheDir(); err == nil && d != "" {
		return d
	}
	return os.TempDir()
}

// ParseCatalog requires both catalog maps, while accepting new upstream fields.
func ParseCatalog(data []byte) (*Catalog, error) {
	var cat Catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return nil, fmt.Errorf("decode catalog: %w", err)
	}
	if cat.Models == nil || cat.Providers == nil {
		return nil, fmt.Errorf("catalog must contain models and providers objects")
	}
	for id, p := range cat.Providers {
		if p.ID == "" {
			p.ID = id
		}
		if p.Name == "" {
			p.Name = p.ID
		}
		if p.Models == nil {
			p.Models = map[string]OfferingModel{}
		}
		for mid, m := range p.Models {
			if m.ID == "" {
				m.ID = mid
			}
			if m.Name == "" {
				m.Name = m.ID
			}
			p.Models[mid] = m
		}
		cat.Providers[id] = p
	}
	for id, m := range cat.Models {
		if m.ID == "" {
			m.ID = id
		}
		if m.Name == "" {
			m.Name = m.ID
		}
		cat.Models[id] = m
	}
	return &cat, nil
}

func (c *Client) cachePath() string { return filepath.Join(c.CacheDir, "catalog.json") }

// atomicWrite uses unique sibling files so failed/concurrent writes cannot
// truncate the previous cache. Rename is atomic on supported Unix platforms.
func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".modeltui-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func (c *Client) writeCache(cat *Catalog) error {
	data, err := json.Marshal(cat)
	if err != nil {
		return err
	}
	return atomicWrite(c.cachePath(), data)
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (c *Client) readCacheWithHash() (*Catalog, string, error) {
	f, err := os.Open(c.cachePath())
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	data, err := readBounded(f, maxCatalogBytes)
	if err != nil {
		return nil, "", err
	}
	cat, err := ParseCatalog(data)
	return cat, digest(data), err
}

func (c *Client) readCache() (*Catalog, error) {
	cat, _, err := c.readCacheWithHash()
	return cat, err
}

func readBounded(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("catalog exceeds %d-byte limit", limit)
	}
	return data, nil
}

func (c *Client) EnsureCacheDir() error { return os.MkdirAll(c.CacheDir, 0755) }

func (c *Client) WarmFromSnapshot() error {
	if _, err := c.readCache(); err == nil {
		return nil
	}
	cat, err := ParseCatalog(snapshotJSON)
	if err != nil {
		return err
	}
	return c.writeCache(cat)
}

// Ping checks the catalog using the same refresh and spacing policy as browsing.
func (c *Client) Ping(ctx context.Context) error { return c.RefreshCatalog(ctx, false).Err }
