package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/desenyon/ModelTUI/internal/catalog"
)

func TestOfflineUsesConfiguredClientAndDisablesRefresh(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer srv.Close()
	c := catalog.NewClient()
	c.BaseURL = srv.URL
	c.CacheDir = t.TempDir()
	c.Offline = true
	m := NewWithClient(context.Background(), c).(model)
	msg := m.bootstrapCatalog()().(loadMsg)
	if msg.err != nil || msg.index == nil || msg.source != "embedded snapshot" {
		t.Fatalf("bootstrap: %+v", msg)
	}
	next, _ := m.Update(msg)
	m = next.(model)
	next, cmd := m.Update(autoRefreshMsg{})
	m = next.(model)
	if cmd != nil || m.refreshing {
		t.Fatal("offline automatic refresh scheduled")
	}
	next, cmd = m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	m = next.(model)
	if cmd != nil || m.refreshing {
		t.Fatal("offline manual refresh scheduled")
	}
	if hits.Load() != 0 {
		t.Fatalf("offline requested network %d times", hits.Load())
	}
}

func TestBootstrapUsesCallerContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := catalog.NewClient()
	c.CacheDir = t.TempDir()
	m := NewWithClient(ctx, c).(model)
	if msg := m.bootstrapCatalog()().(loadMsg); msg.err == nil {
		t.Fatal("ignored canceled context")
	}
}

func TestRefreshTimerGenerationIsReturned(t *testing.T) {
	c := catalog.NewClient()
	c.CacheDir = t.TempDir()
	m := NewWithClient(t.Context(), c).(model)
	m.loading = false
	m.index = catalog.BuildIndex(&catalog.Catalog{Models: map[string]catalog.CanonicalModel{}, Providers: map[string]catalog.Provider{}}, "test")
	next, _ := m.Update(loadMsg{err: context.DeadlineExceeded, silent: true})
	got := next.(model)
	if got.refreshGeneration != m.refreshGeneration+1 {
		t.Fatalf("timer generation not returned: %d", got.refreshGeneration)
	}
	next, cmd := got.Update(autoRefreshMsg{generation: m.refreshGeneration})
	if cmd != nil || next.(model).refreshing {
		t.Fatal("stale timer started another refresh")
	}
}

func TestNavigationAndCapabilities(t *testing.T) {
	c := catalog.NewClient()
	c.CacheDir = t.TempDir()
	c.Offline = true
	m := NewWithClient(t.Context(), c).(model)
	loaded := m.bootstrapCatalog()().(loadMsg)
	next, _ := m.Update(loaded)
	m = next.(model)
	next, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 32})
	m = next.(model)
	next, _ = m.Update(tea.KeyPressMsg{Code: '3', Text: "3"})
	m = next.(model)
	if m.tab != tabOfferings || len(m.lists[tabOfferings].Items()) == 0 {
		t.Fatal("offerings navigation failed")
	}
	m.capsSelected = []string{"reasoning", "tools"}
	m.applyFilters()
	for _, item := range m.lists[tabOfferings].Items() {
		o := item.(browseItem).offering
		if !o.Model.Reasoning || !o.Model.ToolCall {
			t.Fatal("capability intersection failed")
		}
	}
	if m.View().Content == "" {
		t.Fatal("rendered empty screen")
	}
}

func TestFilterFormCommitsSelectionAcrossModelCopies(t *testing.T) {
	c := catalog.NewClient()
	c.CacheDir = t.TempDir()
	c.Offline = true
	m := NewWithClient(t.Context(), c).(model)
	m.loading = false
	next, _ := m.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
	m = next.(model)
	next, _ = m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	m = next.(model)
	m.filterForm.State = huh.StateCompleted
	next, _ = m.Update(nil)
	m = next.(model)
	if len(m.capsSelected) != 1 || m.capsSelected[0] != "reasoning" {
		t.Fatalf("lost form selection: %v", m.capsSelected)
	}
}

func TestBootstrapHTTPTimeoutFallsBack(t *testing.T) {
	for _, cached := range []bool{false, true} {
		name := "snapshot"
		if cached {
			name = "cache"
		}
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
			defer srv.Close()
			c := catalog.NewClient()
			c.BaseURL = srv.URL
			c.CacheDir = t.TempDir()
			c.HTTPClient.Timeout = 20 * time.Millisecond
			if cached {
				if err := os.WriteFile(filepath.Join(c.CacheDir, "catalog.json"), []byte(`{"models":{"lab/m":{"name":"Cached model"}},"providers":{}}`), 0600); err != nil {
					t.Fatal(err)
				}
			}
			m := NewWithClient(t.Context(), c).(model)
			got := m.bootstrapCatalog()().(loadMsg)
			if got.err != nil || got.index == nil {
				t.Fatalf("timeout hid fallback: %+v", got)
			}
			if cached && !strings.HasPrefix(got.source, "disk cache") {
				t.Fatalf("wanted cache: %s", got.source)
			}
			if !cached && got.source != "embedded snapshot" {
				t.Fatalf("wanted snapshot: %s", got.source)
			}
		})
	}
}
