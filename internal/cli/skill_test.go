package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jhdenham/taskship/internal/guide"
)

func TestSkillGuide(t *testing.T) {
	home := tempHome(t)
	out := mustRun(t, "skill", "guide")
	if !strings.HasPrefix(out, guide.Tasks) || !strings.Contains(out, "None yet. The user can add their own in ") {
		t.Errorf("guide without conventions:\n%s", out)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Errorf("skill guide created %s", home)
	}

	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "conventions.md"), []byte("\n- Size every task S, M, or L.\n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out = mustRun(t, "skill", "guide")
	if want := ". Where they differ from the guide above, follow them.\n\n- Size every task S, M, or L.\n"; !strings.HasSuffix(out, want) {
		t.Errorf("guide with conventions doesn't end with %q:\n%s", want, out)
	}
}
