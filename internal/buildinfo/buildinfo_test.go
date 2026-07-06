package buildinfo

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestVersion(t *testing.T) {
	original := rawVersion
	t.Cleanup(func() { rawVersion = original })

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "default", raw: "0.1.0", want: "0.1.0"},
		{name: "blank falls back to default", raw: " \t\n", want: "0.1.0"},
		{name: "trims whitespace", raw: "  1.2.3\n", want: "1.2.3"},
		{name: "removes leading v", raw: "v1.2.3", want: "1.2.3"},
		{name: "removes only one leading v", raw: "vv1.2.3", want: "v1.2.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawVersion = tt.raw
			if got := Version(); got != tt.want {
				t.Fatalf("Version() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVersionFormats(t *testing.T) {
	original := rawVersion
	t.Cleanup(func() { rawVersion = original })
	rawVersion = "0.1.0"

	if got := DisplayVersion(); got != "v0.1.0" {
		t.Fatalf("DisplayVersion() = %q, want %q", got, "v0.1.0")
	}
	if got := UserAgent(); got != "sandrone/0.1.0" {
		t.Fatalf("UserAgent() = %q, want %q", got, "sandrone/0.1.0")
	}
}

func TestMakeAcceptsSafeBuildVersions(t *testing.T) {
	for _, version := range []string{"v0.1.0", "0.1.0-rc.1+meta"} {
		t.Run(version, func(t *testing.T) {
			output, err := runMake(t, "help", "VERSION="+version)
			if err != nil {
				t.Fatalf("make rejected safe VERSION %q: %v\n%s", version, err, output)
			}
		})
	}
}

func TestMakeRejectsUnsafeBuildVersions(t *testing.T) {
	for _, version := range []string{"v0. 1.0", "v0.1.0 ", "v0.1.0\"", "v0.1.0;next", "$HOME", "(v0.1.0)", "v版本"} {
		t.Run(version, func(t *testing.T) {
			if output, err := runMake(t, "help", "VERSION="+version); err == nil {
				t.Fatalf("make accepted unsafe VERSION %q:\n%s", version, output)
			}
		})
	}
}

func TestMakeVersionCannotExecuteMakeFunction(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "make-function-ran")
	value := "$(shell touch " + marker + ")"

	if output, err := runMake(t, "help", "VERSION="+value); err == nil {
		t.Errorf("make accepted executable VERSION:\n%s", output)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("VERSION executed a Make function; marker stat error = %v", err)
	}
}

func TestMakeVersionCannotInjectRecipeCommands(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "recipe-command-ran")
	value := "v0.1.0\"; touch \"" + marker + "\"; #"

	if output, err := runMake(t, "build-probe-mihomo", "GO=false", "VERSION="+value); err == nil {
		t.Errorf("make accepted injectable VERSION:\n%s", output)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("VERSION injected a recipe command; marker stat error = %v", err)
	}
}

func runMake(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command("make", args...)
	cmd.Dir = filepath.Join("..", "..")
	return cmd.CombinedOutput()
}
