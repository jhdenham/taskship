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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// LatestURL redirects to the page of the latest release, which skips
// pre-releases, and DownloadURL is where each release's assets are, under
// its tag. Unlike GitHub's API, neither is rate limited.
var (
	LatestURL   = "https://github.com/jhdenham/taskship/releases/latest"
	DownloadURL = "https://github.com/jhdenham/taskship/releases/download"
)

// maxDownload caps a download's size; a release archive is about 10 MB.
var maxDownload = 200 << 20

var client = &http.Client{Timeout: 5 * time.Minute}

// Latest returns the tag of the latest release and the address of its
// release notes.
func Latest(ctx context.Context) (tag, notes string, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("couldn't check for the latest release at %s: %w", LatestURL, err)
		}
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, LatestURL, nil)
	if err != nil {
		return "", "", err
	}
	noRedirects := *client
	noRedirects.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := noRedirects.Do(req)
	if err != nil {
		return "", "", err
	}
	resp.Body.Close()
	loc, err := resp.Location()
	if err != nil {
		return "", "", errors.New("no release found")
	}
	dir, tag := path.Split(loc.Path)
	if !strings.HasSuffix(dir, "/releases/tag/") || tag == "" {
		return "", "", errors.New("no release found")
	}
	if tag, err = url.PathUnescape(tag); err != nil {
		return "", "", err
	}
	return tag, loc.String(), nil
}

// NotWritableError reports a folder that the new binary can't be written
// to.
type NotWritableError struct {
	Dir string
	Err error
}

func (e *NotWritableError) Error() string { return fmt.Sprintf("can't write to %s: %v", e.Dir, e.Err) }
func (e *NotWritableError) Unwrap() error { return e.Err }

// Install downloads the release tag for this system, checks it against the
// release's checksums, runs it with --version in target's folder, and then
// moves it over target. On any error, including ctx ending, target is left
// as it was and nothing is left behind.
func Install(ctx context.Context, tag, target string) error {
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".taskship-upgrade-*")
	if err != nil {
		return &NotWritableError{dir, err}
	}
	defer os.Remove(tmp.Name()) // fails harmlessly once renamed
	defer tmp.Close()

	archive := fmt.Sprintf("taskship_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	base := DownloadURL + "/" + tag + "/"
	data, err := get(ctx, base+archive)
	if err != nil {
		return err
	}
	sums, err := get(ctx, base+"checksums.txt")
	if err != nil {
		return err
	}
	want, ok := checksum(sums, archive)
	if !ok {
		return fmt.Errorf("checksums.txt has no entry for %s", archive)
	}
	if got := sha256.Sum256(data); hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("checksum mismatch for %s", archive)
	}
	if err := extract(data, tmp); errors.Is(err, errNoBinary) {
		return fmt.Errorf("%s has no taskship binary", archive)
	} else if err != nil {
		return fmt.Errorf("%s: %w", archive, err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	// Run it where it will live (a folder can forbid running programs)
	// before it replaces anything.
	out, err := exec.CommandContext(ctx, tmp.Name(), "--version").Output()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("the downloaded taskship doesn't run here (%v)", err)
	}
	if got := strings.TrimPrefix(strings.TrimSpace(string(out)), "taskship version "); got != tag {
		return fmt.Errorf("the downloaded taskship reports %s, not %s", got, tag)
	}
	return os.Rename(tmp.Name(), target)
}

func get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("couldn't download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("couldn't download %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxDownload)+1))
	switch {
	case ctx.Err() != nil:
		return nil, ctx.Err()
	case err != nil:
		return nil, fmt.Errorf("couldn't download %s: %w", url, err)
	case len(body) > maxDownload:
		return nil, fmt.Errorf("couldn't download %s: it's larger than %d MB", url, maxDownload>>20)
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

var errNoBinary = errors.New("no taskship binary")

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
			return errNoBinary
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
