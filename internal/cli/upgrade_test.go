package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jhdenham/taskship/internal/upgrade"
)

// fakeRelease serves release v0.2.0 with bin as its taskship binary.
type fakeRelease struct {
	bin         []byte
	noArchive   bool // the archive is missing
	badChecksum bool // checksums.txt doesn't match the archive
}

func (f fakeRelease) serve(t *testing.T) {
	t.Helper()
	var tgz bytes.Buffer
	gz := gzip.NewWriter(&tgz)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "taskship", Mode: 0o755, Size: int64(len(f.bin)), Typeflag: tar.TypeReg})
	tw.Write(f.bin)
	tw.Close()
	gz.Close()
	archive := fmt.Sprintf("taskship_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	sum := sha256.Sum256(tgz.Bytes())
	if f.badChecksum {
		sum[0]++
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/latest", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"tag_name": "v0.2.0"}`)
	})
	mux.HandleFunc("/download/v0.2.0/"+archive, func(w http.ResponseWriter, r *http.Request) {
		if f.noArchive {
			http.NotFound(w, r)
			return
		}
		w.Write(tgz.Bytes())
	})
	mux.HandleFunc("/download/v0.2.0/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "%s  taskship_other_arch.tar.gz\n%s  %s\n", strings.Repeat("0", 64), hex.EncodeToString(sum[:]), archive)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	setVar(t, &upgrade.LatestURL, srv.URL+"/latest")
	setVar(t, &upgrade.DownloadURL, srv.URL+"/download")
}

func setVar[T any](t *testing.T, p *T, v T) {
	orig := *p
	*p = v
	t.Cleanup(func() { *p = orig })
}

// oldBinary stands in for the running taskship: a file reached through a
// symlink, as with a binary linked into a folder on PATH. It returns the
// real file.
func oldBinary(t *testing.T) string {
	t.Helper()
	real := filepath.Join(t.TempDir(), "taskship")
	if err := os.WriteFile(real, []byte("old\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "taskship")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	setVar(t, &executable, func() (string, error) { return link, nil })
	return real
}

// assertUntouched checks that the old binary is unchanged and that its
// folder holds nothing else.
func assertUntouched(t *testing.T, real string) {
	t.Helper()
	if b, err := os.ReadFile(real); err != nil || string(b) != "old\n" {
		t.Errorf("old binary changed: %q, %v", b, err)
	}
	if entries, _ := os.ReadDir(filepath.Dir(real)); len(entries) != 1 {
		t.Errorf("%d files next to the binary, want 1", len(entries))
	}
}

// runUpgrade runs taskship upgrade as release version, at a terminal
// typing input if input isn't empty.
func runUpgrade(t *testing.T, version, input string, args ...string) result {
	t.Helper()
	if input != "" {
		setVar(t, &inputIsTerminal, func(io.Reader) bool { return true })
	}
	cmd := NewRootCmd(version)
	var out, errb bytes.Buffer
	cmd.SetIn(strings.NewReader(input))
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	code := Execute(cmd, append([]string{"upgrade"}, args...))
	return result{out.String(), errb.String(), code}
}

const script = "#!/bin/sh\necho taskship version v0.2.0\n"

func TestUpgradeUpToDate(t *testing.T) {
	fakeRelease{bin: []byte(script)}.serve(t)
	real := oldBinary(t)
	for _, v := range []string{"v0.2.0", "v0.3.0"} {
		if out := runUpgrade(t, v, ""); out.code != 0 || out.stdout != "taskship "+v+" is up to date.\n" {
			t.Errorf("%s: %+v", v, out)
		}
	}
	assertUntouched(t, real)
}

func TestUpgradeDeclined(t *testing.T) {
	fakeRelease{bin: []byte(script)}.serve(t)
	real := oldBinary(t)
	out := runUpgrade(t, "v0.1.0", "n\n")
	if out.code != 0 || out.stdout != "Current: v0.1.0\nLatest:  v0.2.0\nNothing changed.\n" || !strings.Contains(out.stderr, "Upgrade? (y/N)") {
		t.Errorf("declined: %+v", out)
	}
	assertUntouched(t, real)
}

func TestUpgradeFromPreRelease(t *testing.T) {
	fakeRelease{bin: []byte(script)}.serve(t)
	real := oldBinary(t)
	out := runUpgrade(t, "v0.2.0-rc.1", "", "--yes")
	if out.code != 0 || !strings.Contains(out.stdout, "✓ Upgraded taskship v0.2.0-rc.1 → v0.2.0\n") {
		t.Errorf("rc to final: %+v", out)
	}
	if b, _ := os.ReadFile(real); string(b) != script {
		t.Errorf("binary not replaced: %q", b)
	}
}

func TestUpgradeNeedsYesWithoutTerminal(t *testing.T) {
	fakeRelease{bin: []byte(script)}.serve(t)
	real := oldBinary(t)
	if out := runUpgrade(t, "v0.1.0", ""); out.code != exitInvalidInput || !strings.Contains(out.stderr, "pass --yes") {
		t.Errorf("no terminal: %+v", out)
	}
	assertUntouched(t, real)
}

func TestUpgradeFromSource(t *testing.T) {
	setVar(t, &upgrade.LatestURL, "http://127.0.0.1:0/never") // no network
	real := oldBinary(t)
	out := runUpgrade(t, "", "", "--yes")
	if out.code != exitInvalidInput || !strings.Contains(out.stderr, "go install github.com/jhdenham/taskship/cmd/taskship@latest") {
		t.Errorf("from source: %+v", out)
	}
	assertUntouched(t, real)
}

func TestUpgradeFailures(t *testing.T) {
	cases := map[string]struct {
		release fakeRelease
		want    string
	}{
		"missing archive":   {fakeRelease{bin: []byte(script), noArchive: true}, "404 Not Found"},
		"checksum mismatch": {fakeRelease{bin: []byte(script), badChecksum: true}, "checksum mismatch"},
		"doesn't run":       {fakeRelease{bin: []byte("#!/bin/sh\nexit 1\n")}, "doesn't run here"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			c.release.serve(t)
			real := oldBinary(t)
			out := runUpgrade(t, "v0.1.0", "", "--yes")
			if out.code != exitFailure || !strings.Contains(out.stderr, c.want) || !strings.Contains(out.stderr, "taskship v0.1.0 is still installed") {
				t.Errorf("%+v", out)
			}
			assertUntouched(t, real)
		})
	}

	t.Run("folder not writable", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root can write anywhere")
		}
		fakeRelease{bin: []byte(script)}.serve(t)
		real := oldBinary(t)
		dir := filepath.Dir(real)
		os.Chmod(dir, 0o555)
		t.Cleanup(func() { os.Chmod(dir, 0o755) })
		if out := runUpgrade(t, "v0.1.0", "", "--yes"); out.code != exitFailure || !strings.Contains(out.stderr, "can't write to") {
			t.Errorf("%+v", out)
		}
		assertUntouched(t, real)
	})
}

func TestUpgrade(t *testing.T) {
	newBin := filepath.Join(t.TempDir(), "taskship")
	build := exec.Command("go", "build", "-ldflags", "-X main.version=v0.2.0", "-o", newBin, "../../cmd/taskship")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	b, err := os.ReadFile(newBin)
	if err != nil {
		t.Fatal(err)
	}
	fakeRelease{bin: b}.serve(t)
	real := oldBinary(t)

	tempHome(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	if err := os.WriteFile(filepath.Join(bin, "taskship"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out := run(t, "v0.1.0", "skill", "install", "claude"); out.code != 0 {
		t.Fatalf("install skill: %+v", out)
	}

	out := runUpgrade(t, "v0.1.0", "y\n")
	want := "Current: v0.1.0\nLatest:  v0.2.0\n" +
		"✓ Upgraded taskship v0.1.0 → v0.2.0\n" +
		"✓ Updated the Taskship skill for Claude (v0.1.0 → v0.2.0)\n"
	if out.code != 0 || out.stdout != want {
		t.Fatalf("upgrade: %+v\nwant stdout:\n%s", out, want)
	}
	if v, err := exec.Command(real, "--version").Output(); err != nil || string(v) != "taskship version v0.2.0\n" {
		t.Errorf("new --version: %q, %v", v, err)
	}
	if fi, err := os.Lstat(must(executable())); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("symlink replaced: %v, %v", fi, err)
	}
	list := run(t, "v0.2.0", "skill", "list").stdout
	wantList := "AGENT   SKILL          VERSION  FOLDER\n" +
		"claude  installed      v0.2.0   ~/.claude/skills/taskship\n" +
		"codex   not installed           ~/.agents/skills/taskship\n"
	if list != wantList {
		t.Errorf("skill list:\n%s\nwant:\n%s", list, wantList)
	}
}

func must(s string, err error) string {
	if err != nil {
		panic(err)
	}
	return s
}
