// Package upgrade replaces the taskship binary with the latest GitHub
// release, with the same checks as install.sh.
package upgrade

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// LatestURL returns the latest release, which skips pre-releases, and
// DownloadURL is where each release's assets are, under the tag.
var (
	LatestURL   = "https://api.github.com/repos/jhdenham/taskship/releases/latest"
	DownloadURL = "https://github.com/jhdenham/taskship/releases/download"
)

var client = &http.Client{Timeout: 5 * time.Minute}

// Latest returns the tag of the latest release.
func Latest(ctx context.Context) (string, error) {
	body, err := get(ctx, LatestURL)
	if err != nil {
		return "", err
	}
	var release struct {
		Tag string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &release); err != nil || release.Tag == "" {
		return "", fmt.Errorf("couldn't read the latest release from %s", LatestURL)
	}
	return release.Tag, nil
}

// Install downloads the release tag for this system, checks it against the
// release's checksums, runs it with --version in target's folder, and then
// moves it over target. It returns what --version printed. On any error,
// target is left as it was.
func Install(ctx context.Context, tag, target string) (string, error) {
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".taskship-upgrade-*")
	if err != nil {
		return "", fmt.Errorf("can't write to %s: %w", dir, err)
	}
	defer os.Remove(tmp.Name()) // fails harmlessly once renamed
	defer tmp.Close()

	archive := fmt.Sprintf("taskship_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	base := DownloadURL + "/" + tag + "/"
	data, err := get(ctx, base+archive)
	if err != nil {
		return "", err
	}
	sums, err := get(ctx, base+"checksums.txt")
	if err != nil {
		return "", err
	}
	want, ok := checksum(sums, archive)
	if !ok {
		return "", fmt.Errorf("checksums.txt has no entry for %s", archive)
	}
	if got := sha256.Sum256(data); hex.EncodeToString(got[:]) != want {
		return "", fmt.Errorf("checksum mismatch for %s", archive)
	}
	if err := extract(data, tmp); err != nil {
		return "", fmt.Errorf("%s: %w", archive, err)
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return "", err
	}
	// Run it where it will live (a folder can forbid running programs)
	// before it replaces anything.
	out, err := exec.CommandContext(ctx, tmp.Name(), "--version").CombinedOutput()
	version := strings.TrimSpace(string(out))
	if err != nil {
		if version != "" {
			err = fmt.Errorf("%w: %s", err, version)
		}
		return "", fmt.Errorf("the downloaded taskship doesn't run here (%v)", err)
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		return "", err
	}
	return version, nil
}

func get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("couldn't download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("couldn't download %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("couldn't download %s: %w", url, err)
	}
	return body, nil
}

// checksum finds name's sha256 in sums, which is in sha256sum format.
func checksum(sums []byte, name string) (string, bool) {
	s := bufio.NewScanner(bytes.NewReader(sums))
	for s.Scan() {
		if f := strings.Fields(s.Text()); len(f) == 2 && f[1] == name {
			return f[0], true
		}
	}
	return "", false
}

// extract writes the top-level taskship file in the tar.gz data to w.
func extract(data []byte, w io.Writer) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return errors.New("no taskship binary in the archive")
		}
		if err != nil {
			return err
		}
		if h.Name == "taskship" && h.Typeflag == tar.TypeReg {
			_, err := io.Copy(w, tr)
			return err
		}
	}
}
