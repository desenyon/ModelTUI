package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	defaultRepo = "desenyon/ModelTUI"
	userAgent   = "modeltui-updater/1.0"
)

// Version is set via -ldflags at build time.
var Version = "dev"

type Result struct {
	Current   string
	Latest    string
	AssetURL  string
	AssetName string
	UpToDate  bool
	CheckedAt time.Time
}

type release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// Check looks up the latest stable release for the current platform.
func Check(ctx context.Context, repo string) (Result, error) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return Result{}, fmt.Errorf("self-update supports macOS and Linux; build from source on %s", runtime.GOOS)
	}
	if repo == "" {
		repo = defaultRepo
	}
	return checkRelease(ctx, &http.Client{Timeout: 20 * time.Second}, "https://api.github.com/repos/"+repo+"/releases/latest", Version)
}

func parseVersion(v string) ([3]uint64, error) {
	var out [3]uint64
	if v == "dev" {
		return out, fmt.Errorf("development build: update with go install or rebuild from source")
	}
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return out, fmt.Errorf("unsupported version %q: expected stable major.minor.patch", v)
	}
	for i, p := range parts {
		if p == "" || (len(p) > 1 && p[0] == '0') {
			return out, fmt.Errorf("invalid version %q", v)
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return out, fmt.Errorf("unsupported version %q: expected stable major.minor.patch", v)
			}
		}
		n, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return out, fmt.Errorf("invalid version %q: %w", v, err)
		}
		out[i] = n
	}
	return out, nil
}

func checkRelease(ctx context.Context, client *http.Client, url, current string) (Result, error) {
	currentVersion, err := parseVersion(current)
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 256))
		return Result{}, fmt.Errorf("GitHub releases: %s (%s)", res.Status, string(body))
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	if err != nil {
		return Result{}, err
	}
	if len(data) > 1<<20 {
		return Result{}, fmt.Errorf("release metadata exceeds 1 MiB")
	}
	var rel release
	if err := json.Unmarshal(data, &rel); err != nil {
		return Result{}, err
	}
	latestVersion, err := parseVersion(rel.TagName)
	if err != nil {
		return Result{}, err
	}
	upToDate := true
	for i := range currentVersion {
		if currentVersion[i] != latestVersion[i] {
			upToDate = currentVersion[i] > latestVersion[i]
			break
		}
	}
	latest := strings.TrimPrefix(rel.TagName, "v")
	out := Result{Current: strings.TrimPrefix(current, "v"), Latest: latest, UpToDate: upToDate, CheckedAt: time.Now()}
	want := assetName(latest)
	for _, a := range rel.Assets {
		if a.Name == want {
			out.AssetURL, out.AssetName = a.BrowserDownloadURL, a.Name
			break
		}
	}
	if out.AssetURL == "" && !out.UpToDate {
		return out, fmt.Errorf("no asset %s in release %s", want, rel.TagName)
	}
	return out, nil
}

func assetName(version string) string {
	return fmt.Sprintf("modeltui_%s_%s_%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)
}
