package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseAssetNameMatchesPublishedArchive(t *testing.T) {
	want := fmt.Sprintf("modeltui_1.2.3_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	if got := assetName("1.2.3"); got != want {
		t.Fatalf("asset %q, want %q", got, want)
	}
}

func TestCheckReleaseSelectsArchiveAndNeverDowngrades(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing user agent")
		}
		fmt.Fprintf(w, `{"tag_name":"v1.2.3","assets":[{"name":%q,"browser_download_url":"https://example.org/release.tar.gz"}]}`, assetName("1.2.3"))
	}))
	defer srv.Close()
	for _, tc := range []struct {
		current string
		up      bool
	}{{"1.0.0", false}, {"v1.2.3", true}, {"1.10.0", true}, {"2.0.0", true}} {
		got, err := checkRelease(t.Context(), srv.Client(), srv.URL, tc.current)
		if err != nil || got.UpToDate != tc.up || got.AssetURL == "" {
			t.Fatalf("%s: %+v %v", tc.current, got, err)
		}
	}
	for _, version := range []string{"dev", "broken", "1.2.3-beta"} {
		if _, err := checkRelease(t.Context(), srv.Client(), srv.URL, version); err == nil {
			t.Fatalf("accepted %s", version)
		}
	}
}

func archive(t *testing.T, name string, kind byte, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	size := int64(len(payload))
	if kind != tar.TypeReg {
		size = 0
		payload = nil
	}
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0755, Size: size, Typeflag: kind}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestApplyArchiveAtomicallyAndPreserveOnFailure(t *testing.T) {
	valid := archive(t, "modeltui", tar.TypeReg, []byte("new binary"))
	badChecksum := append([]byte(nil), valid...)
	badChecksum[len(badChecksum)-5] ^= 0xff
	for _, tc := range []struct {
		name    string
		data    []byte
		success bool
	}{
		{"valid", valid, true}, {"raw", []byte("raw executable"), false},
		{"traversal", archive(t, "../modeltui", tar.TypeReg, []byte("bad")), false},
		{"symlink", archive(t, "modeltui", tar.TypeSymlink, nil), false},
		{"empty", archive(t, "modeltui", tar.TypeReg, nil), false},
		{"checksum", badChecksum, false}, {"truncated", valid[:len(valid)/2], false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			exe := filepath.Join(dir, "modeltui")
			if err := os.WriteFile(exe, []byte("old binary"), 0755); err != nil {
				t.Fatal(err)
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(tc.data) }))
			defer srv.Close()
			err := applyTo(t.Context(), srv.Client(), srv.URL, exe)
			if (err == nil) != tc.success {
				t.Fatalf("success=%v err=%v", tc.success, err)
			}
			got, err := os.ReadFile(exe)
			if err != nil {
				t.Fatal(err)
			}
			want := "old binary"
			if tc.success {
				want = "new binary"
			}
			if string(got) != want {
				t.Fatalf("binary was %q", got)
			}
			files, err := os.ReadDir(dir)
			if err != nil || len(files) != 1 {
				t.Fatalf("temporary files leaked: %v %v", files, err)
			}
		})
	}
}

func TestApplyCanceledDoesNotReplace(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "modeltui")
	os.WriteFile(exe, []byte("old"), 0755)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := applyTo(ctx, http.DefaultClient, "https://example.org/archive", exe); err == nil {
		t.Fatal("ignored cancellation")
	}
	got, _ := os.ReadFile(exe)
	if strings.TrimSpace(string(got)) != "old" {
		t.Fatal("replaced executable")
	}
}
