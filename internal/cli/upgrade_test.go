package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
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

	"github.com/jhdenham/taskship/internal/display"
	"github.com/jhdenham/taskship/internal/session"
	"github.com/jhdenham/taskship/internal/upgrade"
)

// fakeRelease serves release v0.2.0 with bin as its taskship binary.
type fakeRelease struct {
	bin         []byte
	noRelease   bool   // there is no latest release
	status      int    // if set, the latest release lookup fails with it
	noArchive   bool   // the archive is missing
	badChecksum bool   // checksums.txt doesn't match the archive
	noChecksum  bool   // checksums.txt has no entry for the archive
	requested   *bool  // set when the archive is requested
	name        string // the binary's name in the archive, if not taskship
	onArchive   func() // called when the archive is requested
}

func (f fakeRelease) serve(t *testing.T) {
	t.Helper()
	name := f.name
	if name == "" {
		name = "taskship"
	}
	var tgz bytes.Buffer
	gz := gzip.NewWriter(&tgz)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(f.bin)), Typeflag: tar.TypeReg})
	tw.Write(f.bin)
	tw.Close()
	gz.Close()
	archive := fmt.Sprintf("taskship_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	sum := sha256.Sum256(tgz.Bytes())
	if f.badChecksum {
		sum[0]++
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if f.status != 0 {
			w.WriteHeader(f.status)
			return
		}
		if f.noRelease {
			http.Redirect(w, r, "/releases", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/releases/tag/v0.2.0", http.StatusFound)
	})
	mux.HandleFunc("/download/v0.2.0/"+archive, func(w http.ResponseWriter, r *http.Request) {
		if f.requested != nil {
			*f.requested = true
		}
		if f.onArchive != nil {
			f.onArchive()
			<-r.Context().Done()
			return
		}
		if f.noArchive {
			http.NotFound(w, r)
			return
		}
		w.Write(tgz.Bytes())
	})
	mux.HandleFunc("/download/v0.2.0/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		if f.noChecksum {
			fmt.Fprintf(w, "%s  taskship_other_arch.tar.gz\n", strings.Repeat("0", 64))
			return
		}
		fmt.Fprintf(w, "%s  taskship_other_arch.tar.gz\n%s  %s\n", strings.Repeat("0", 64), hex.EncodeToString(sum[:]), archive)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	setVar(t, &upgrade.LatestURL, srv.URL+"/releases/latest")
	setVar(t, &upgrade.DownloadURL, srv.URL+"/download")
	notesURL = srv.URL + "/releases/tag/v0.2.0"
}

// notesURL is the release notes address the latest fakeRelease serves.
var notesURL string

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
	return must(filepath.EvalSymlinks(real)) // as upgrade shows it: /var is a symlink on macOS
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

// runUpgrade runs taskship upgrade as release version with ctx, at a
// terminal typing input if input isn't empty.
func runUpgrade(t *testing.T, ctx context.Context, version, input string, args ...string) result {
	t.Helper()
	if input != "" {
		setVar(t, &inputIsTerminal, func(io.Reader) bool { return true })
	}
	cmd := NewRootCmd(version)
	var out, errb bytes.Buffer
	cmd.SetContext(ctx)
	cmd.SetIn(strings.NewReader(input))
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	code := Execute(cmd, append([]string{"upgrade"}, args...))
	return result{out.String(), errb.String(), code}
}

// script is a stand-in release binary that reports version.
func script(version string) []byte {
	return []byte("#!/bin/sh\necho taskship version " + version + "\n")
}

func TestUpgradeUpToDate(t *testing.T) {
	tempHome(t)
	fakeRelease{bin: script("v0.2.0")}.serve(t)
	real := oldBinary(t)
	for _, v := range []string{"v0.2.0", "v0.3.0"} {
		if out := runUpgrade(t, t.Context(), v, ""); out.code != 0 || out.stdout != "taskship "+v+" is up to date.\n" {
			t.Errorf("%s: %+v", v, out)
		}
	}
	assertUntouched(t, real)
}

func TestUpgradeDeclined(t *testing.T) {
	tempHome(t)
	fakeRelease{bin: script("v0.2.0")}.serve(t)
	real := oldBinary(t)
	out := runUpgrade(t, t.Context(), "v0.1.0", "n\n")
	want := "Current: v0.1.0\nLatest:  v0.2.0\nRelease notes: " + notesURL + "\nNothing changed.\n"
	if out.code != 0 || out.stdout != want || !strings.Contains(out.stderr, "Upgrade? (y/N)") {
		t.Errorf("declined: %+v\nwant stdout:\n%s", out, want)
	}
	assertUntouched(t, real)
}

func TestUpgradeFromPreRelease(t *testing.T) {
	tempHome(t)
	fakeRelease{bin: script("v0.2.0")}.serve(t)
	real := oldBinary(t)
	out := runUpgrade(t, t.Context(), "v0.2.0-rc.1", "", "--yes")
	if out.code != 0 || !strings.Contains(out.stdout, "Downloading taskship v0.2.0…\n✓ Upgraded taskship v0.2.0-rc.1 → v0.2.0\n") {
		t.Errorf("rc to final: %+v", out)
	}
	if b, _ := os.ReadFile(real); string(b) != string(script("v0.2.0")) {
		t.Errorf("binary not replaced: %q", b)
	}
}

func TestUpgradeNeedsYesWithoutTerminal(t *testing.T) {
	tempHome(t)
	fakeRelease{bin: script("v0.2.0")}.serve(t)
	real := oldBinary(t)
	if out := runUpgrade(t, t.Context(), "v0.1.0", ""); out.code != exitInvalidInput || out.stderr != "taskship: pass --yes to upgrade without a terminal\n" {
		t.Errorf("no terminal: %+v", out)
	}
	assertUntouched(t, real)
}

func TestUpgradeFromSource(t *testing.T) {
	setVar(t, &upgrade.LatestURL, "http://127.0.0.1:0/never") // no network
	real := oldBinary(t)
	path := display.Path(real)

	setVar(t, &builtFromCheckout, func() bool { return true })
	out := runUpgrade(t, t.Context(), "", "", "--yes")
	want := "taskship: " + path + " was built from a source checkout, so upgrade won't replace it\n\nTo update it, pull the latest changes and rebuild it.\n"
	if out.code != exitInvalidInput || out.stderr != want {
		t.Errorf("checkout: %+v\nwant stderr:\n%s", out, want)
	}

	builtFromCheckout = func() bool { return false }
	out = runUpgrade(t, t.Context(), "", "", "--yes")
	want = "taskship: " + path + " was installed with go install, so upgrade won't replace it\n\nUpdate it with:\ngo install github.com/jhdenham/taskship/cmd/taskship@latest\n"
	if out.code != exitInvalidInput || out.stderr != want {
		t.Errorf("go install: %+v\nwant stderr:\n%s", out, want)
	}
	assertUntouched(t, real)
}

func TestUpgradeNoRelease(t *testing.T) {
	tempHome(t)
	fakeRelease{noRelease: true}.serve(t)
	real := oldBinary(t)
	out := runUpgrade(t, t.Context(), "v0.1.0", "", "--yes")
	want := "taskship: couldn't check for the latest release at " + upgrade.LatestURL + ": no release found\n"
	if out.code != exitFailure || out.stderr != want {
		t.Errorf("%+v\nwant stderr:\n%s", out, want)
	}
	assertUntouched(t, real)
}

func TestUpgradeLookupFails(t *testing.T) {
	tempHome(t)
	fakeRelease{status: http.StatusTooManyRequests}.serve(t)
	real := oldBinary(t)
	out := runUpgrade(t, t.Context(), "v0.1.0", "", "--yes")
	want := "taskship: couldn't check for the latest release at " + upgrade.LatestURL + ": 429 Too Many Requests\n"
	if out.code != exitFailure || out.stderr != want {
		t.Errorf("%+v\nwant stderr:\n%s", out, want)
	}
	assertUntouched(t, real)
}

// A release without a checksum for this system fails before the archive
// is downloaded.
func TestUpgradeNoChecksumEntry(t *testing.T) {
	tempHome(t)
	requested := false
	fakeRelease{bin: script("v0.2.0"), noChecksum: true, requested: &requested}.serve(t)
	real := oldBinary(t)
	out := runUpgrade(t, t.Context(), "v0.1.0", "", "--yes")
	archive := fmt.Sprintf("taskship_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	want := "taskship: checksums.txt has no entry for " + archive + "\n\nNothing changed: taskship v0.1.0 is still installed.\n"
	if out.code != exitFailure || out.stderr != want || requested {
		t.Errorf("%+v, archive requested: %v\nwant stderr:\n%s", out, requested, want)
	}
	assertUntouched(t, real)
}

// A skill that can't be read after the binary is replaced is reported as
// a skill problem, not a failed upgrade.
func TestUpgradeSkillUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read anything")
	}
	tempHome(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	dir := filepath.Join(home, ".claude", "skills", "taskship")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("x"), 0o000)
	fakeRelease{bin: script("v0.2.0")}.serve(t)
	real := oldBinary(t)
	out := runUpgrade(t, t.Context(), "v0.1.0", "", "--yes")
	if out.code != exitFailure || !strings.Contains(out.stdout, "✓ Upgraded taskship v0.1.0 → v0.2.0\n") ||
		!strings.HasPrefix(out.stderr, "taskship: taskship was upgraded to v0.2.0, but updating the skill for Claude failed (") ||
		!strings.HasSuffix(out.stderr, "permission denied)\n\nTo update it, run: taskship skill install claude\n") {
		t.Errorf("%+v", out)
	}
	if b, _ := os.ReadFile(real); string(b) != string(script("v0.2.0")) {
		t.Errorf("binary not replaced: %q", b)
	}
}

func TestUpgradeFailures(t *testing.T) {
	archive := fmt.Sprintf("taskship_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	cases := map[string]struct {
		release fakeRelease
		want    string // the error's first line
	}{
		"missing archive":   {fakeRelease{bin: script("v0.2.0"), noArchive: true}, "couldn't download %s/download/v0.2.0/" + archive + ": 404 Not Found"},
		"checksum mismatch": {fakeRelease{bin: script("v0.2.0"), badChecksum: true}, "checksum mismatch for " + archive},
		"no binary":         {fakeRelease{bin: script("v0.2.0"), name: "bin/taskship"}, archive + " has no taskship binary"},
		"doesn't run":       {fakeRelease{bin: []byte("#!/bin/sh\nexit 1\n")}, "the downloaded taskship doesn't run here (exit status 1)"},
		"wrong version":     {fakeRelease{bin: script("v0.1.9")}, "the downloaded taskship reports v0.1.9, not v0.2.0"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			tempHome(t)
			c.release.serve(t)
			real := oldBinary(t)
			out := runUpgrade(t, t.Context(), "v0.1.0", "", "--yes")
			first := c.want
			if strings.Contains(first, "%s") {
				first = fmt.Sprintf(first, strings.TrimSuffix(upgrade.DownloadURL, "/download"))
			}
			want := "taskship: " + first + "\n\nNothing changed: taskship v0.1.0 is still installed.\n"
			if out.code != exitFailure || out.stderr != want {
				t.Errorf("%+v\nwant stderr:\n%s", out, want)
			}
			assertUntouched(t, real)
		})
	}

	t.Run("folder not writable", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root can write anywhere")
		}
		tempHome(t)
		fakeRelease{bin: script("v0.2.0")}.serve(t)
		real := oldBinary(t)
		dir := filepath.Dir(real)
		os.Chmod(dir, 0o555)
		t.Cleanup(func() { os.Chmod(dir, 0o755) })
		out := runUpgrade(t, t.Context(), "v0.1.0", "", "--yes")
		want := "taskship: can't write to " + display.Path(dir) + ", where taskship is installed\n\n" +
			"Nothing changed: taskship v0.1.0 is still installed. Upgrade it the way you installed it there, or reinstall in a folder you own with:\n" +
			"curl -fsSL https://taskship.dev/install.sh | sh\n"
		if out.code != exitFailure || out.stderr != want {
			t.Errorf("%+v\nwant stderr:\n%s", out, want)
		}
		assertUntouched(t, real)
	})

	t.Run("interrupted", func(t *testing.T) {
		tempHome(t)
		ctx, cancel := context.WithCancel(t.Context())
		fakeRelease{bin: script("v0.2.0"), onArchive: cancel}.serve(t)
		real := oldBinary(t)
		out := runUpgrade(t, ctx, "v0.1.0", "", "--yes")
		want := "taskship: upgrade interrupted\n\nNothing changed: taskship v0.1.0 is still installed.\n"
		if out.code != exitFailure || out.stderr != want {
			t.Errorf("%+v\nwant stderr:\n%s", out, want)
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

	taskshipHome := tempHome(t)
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
	stop, err := session.Start(taskshipHome, "taskship run PROJ-42")
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	out := runUpgrade(t, t.Context(), "v0.1.0", "y\n")
	want := "Current: v0.1.0\nLatest:  v0.2.0\nRelease notes: " + notesURL + "\n" +
		"Downloading taskship v0.2.0…\n" +
		"✓ Upgraded taskship v0.1.0 → v0.2.0\n" +
		"✓ Updated the Taskship skill for Claude (v0.1.0 → v0.2.0)\n" +
		"You have a running Taskship session that needs to be restarted to use v0.2.0:\n" +
		fmt.Sprintf("  taskship run PROJ-42   pid %d\n", os.Getpid())
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

func TestUpgradeListsSessions(t *testing.T) {
	taskshipHome := tempHome(t)
	fakeRelease{bin: script("v0.2.0")}.serve(t)
	oldBinary(t)
	for _, label := range []string{"taskship", "taskship refine PROJ-7"} {
		stop, err := session.Start(taskshipHome, label)
		if err != nil {
			t.Fatal(err)
		}
		// Both run in this process, so the second record replaces the
		// first; record the first again under another name to list two.
		defer stop()
		if label == "taskship" {
			os.Rename(filepath.Join(taskshipHome, session.Dir, fmt.Sprint(os.Getpid())), filepath.Join(taskshipHome, session.Dir, "1"))
		}
	}
	out := runUpgrade(t, t.Context(), "v0.1.0", "", "--yes")
	want := "You have running Taskship sessions that need to be restarted to use v0.2.0:\n" +
		"  taskship                 pid 1\n" +
		fmt.Sprintf("  taskship refine PROJ-7   pid %d\n", os.Getpid())
	if out.code != 0 || !strings.HasSuffix(out.stdout, want) {
		t.Errorf("%+v\nwant stdout ending:\n%s", out, want)
	}
}

func TestSessionsRecorded(t *testing.T) {
	taskshipHome := tempHome(t)
	// Without a terminal the TUI doesn't open, so record the way runTUI
	// does and check the label.
	cmd := NewRootCmd("")
	stop := startSession(cmd, "")
	sessions, err := session.Open(taskshipHome)
	stop()
	if err != nil || len(sessions) != 1 || sessions[0].Label != "taskship" {
		t.Errorf("sessions = %v, %v", sessions, err)
	}
	run, _, _ := cmd.Find([]string{"run"})
	stop = startSession(run, "PROJ-42")
	sessions, _ = session.Open(taskshipHome)
	stop()
	if len(sessions) != 1 || sessions[0].Label != "taskship run PROJ-42" {
		t.Errorf("sessions = %v", sessions)
	}
}

func must(s string, err error) string {
	if err != nil {
		panic(err)
	}
	return s
}
