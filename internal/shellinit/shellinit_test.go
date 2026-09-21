package shellinit

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestScriptForEachShell(t *testing.T) {
	for _, shell := range []string{"zsh", "bash", "fish"} {
		got, err := Script(shell)
		if err != nil {
			t.Fatalf("Script(%q): %v", shell, err)
		}
		if !strings.Contains(got, "DL_CD_FILE") {
			t.Errorf("%s script does not set DL_CD_FILE", shell)
		}
		if !strings.Contains(got, "command dl") {
			t.Errorf("%s script does not call the binary with `command`, so it would recurse", shell)
		}
	}
}

func TestScriptRejectsAnUnknownShell(t *testing.T) {
	if _, err := Script("csh"); err == nil {
		t.Fatal("Script(\"csh\") = nil error, want an error")
	}
}

// The emitted shell code must actually be valid. Skip a shell that is not
// installed rather than failing on a machine that lacks it.
func TestScriptsAreSyntacticallyValid(t *testing.T) {
	cases := []struct {
		shell, bin string
		args       []string
	}{
		{"bash", "bash", []string{"-n"}},
		{"zsh", "zsh", []string{"-n"}},
		{"fish", "fish", []string{"--no-execute"}},
	}
	for _, c := range cases {
		t.Run(c.shell, func(t *testing.T) {
			bin, err := exec.LookPath(c.bin)
			if err != nil {
				t.Skipf("%s is not installed", c.bin)
			}
			script, err := Script(c.shell)
			if err != nil {
				t.Fatal(err)
			}
			f := t.TempDir() + "/init"
			if err := os.WriteFile(f, []byte(script), 0o644); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command(bin, append(c.args, f)...).CombinedOutput()
			if err != nil {
				t.Fatalf("%s syntax check failed: %v\n%s", c.shell, err, out)
			}
		})
	}
}
