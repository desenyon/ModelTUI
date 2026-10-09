package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	maxArchiveBytes int64 = 64 << 20
	maxBinaryBytes  int64 = 128 << 20
)

// Apply replaces the current executable only after validating the entire archive.
func Apply(ctx context.Context, assetURL string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}
	return applyTo(ctx, &http.Client{Timeout: 3 * time.Minute}, assetURL, exe)
}

func applyTo(ctx context.Context, client *http.Client, url, exe string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", res.Status)
	}
	dir := filepath.Dir(exe)
	archive, err := os.CreateTemp(dir, ".modeltui-archive-*")
	if err != nil {
		return err
	}
	defer os.Remove(archive.Name())
	defer archive.Close()
	n, err := io.Copy(archive, io.LimitReader(res.Body, maxArchiveBytes+1))
	if err != nil {
		return err
	}
	if n > maxArchiveBytes {
		return fmt.Errorf("update archive exceeds 64 MiB")
	}
	if _, err = archive.Seek(0, io.SeekStart); err != nil {
		return err
	}
	gz, err := gzip.NewReader(archive)
	if err != nil {
		return fmt.Errorf("invalid update gzip: %w", err)
	}
	defer gz.Close()
	limited := &io.LimitedReader{R: gz, N: maxBinaryBytes + (1 << 20)}
	tr := tar.NewReader(limited)
	binary, err := os.CreateTemp(dir, ".modeltui-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(binary.Name())
	defer binary.Close()
	found := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid update tar: %w", err)
		}
		if found || hdr.Name != "modeltui" || hdr.Typeflag != tar.TypeReg || hdr.Size <= 0 || hdr.Size > maxBinaryBytes {
			return fmt.Errorf("archive must contain exactly one nonempty regular modeltui binary (128 MiB maximum)")
		}
		if _, err = io.Copy(binary, tr); err != nil {
			return fmt.Errorf("extract update: %w", err)
		}
		found = true
	}
	if !found {
		return fmt.Errorf("archive contains no modeltui binary")
	}
	// tar EOF alone does not prove the gzip checksum/trailer was consumed.
	if _, err = io.Copy(io.Discard, limited); err != nil {
		return fmt.Errorf("invalid gzip trailer: %w", err)
	}
	if limited.N == 0 {
		return fmt.Errorf("expanded archive exceeds size limit")
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = binary.Chmod(0755); err != nil {
		return err
	}
	if err = binary.Sync(); err != nil {
		return err
	}
	if err = binary.Close(); err != nil {
		return err
	}
	// On supported Unix platforms this atomically replaces the destination; an
	// unsuccessful rename leaves the original executable in place.
	if err = os.Rename(binary.Name(), exe); err != nil {
		return fmt.Errorf("replace executable: %w", err)
	}
	return nil
}
