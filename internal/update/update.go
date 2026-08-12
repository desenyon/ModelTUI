package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultRepo = "desenyon/ModelTUI"
	userAgent   = "modeltui-updater/1.0"
)

// Version is set via -ldflags at build time.
var Version = "dev"

// Result describes an available update.
type Result struct {
	Current    string
	Latest     string
	AssetURL   string
	AssetName  string
	UpToDate   bool
	CheckedAt  time.Time
}

type release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// Check looks up the latest GitHub release for this OS/arch.
func Check(ctx context.Context, repo string) (Result, error) {
	if repo == "" {
		repo = defaultRepo
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 256))
		return Result{}, fmt.Errorf("github releases: %s (%s)", res.Status, string(body))
	}
	var rel release
	if err := json.NewDecoder(res.Body).Decode(&rel); err != nil {
		return Result{}, err
	}
	latest := strings.TrimPrefix(rel.TagName, "v")
	current := strings.TrimPrefix(Version, "v")
	out := Result{
		Current:   current,
		Latest:    latest,
		UpToDate:  current == latest || current == "dev",
		CheckedAt: time.Now(),
	}
	want := assetName(latest)
	for _, a := range rel.Assets {
		if a.Name == want {
			out.AssetURL = a.BrowserDownloadURL
			out.AssetName = a.Name
			break
		}
	}
	if out.AssetURL == "" && !out.UpToDate {
		return out, fmt.Errorf("no asset %s in release %s", want, rel.TagName)
	}
	return out, nil
}

func assetName(version string) string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}
	return fmt.Sprintf("modeltui_%s_%s_%s%s", version, goos, goarch, ext)
}

// Apply downloads AssetURL and replaces the current executable.
func Apply(ctx context.Context, assetURL string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	client := &http.Client{Timeout: 3 * time.Minute}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", res.Status)
	}

	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, "modeltui-update-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := io.Copy(tmp, res.Body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	backup := exe + ".bak"
	_ = os.Remove(backup)
	if err := os.Rename(exe, backup); err != nil {
		return err
	}
	if err := os.Rename(tmpName, exe); err != nil {
		_ = os.Rename(backup, exe)
		return err
	}
	_ = os.Remove(backup)
	return nil
}
