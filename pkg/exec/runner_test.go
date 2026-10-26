package exec

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// Run replaces the current process, so the test binary re-executes itself as parent, then child.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "child" {
		fmt.Print(os.Getenv("SHARED"))
		os.Exit(7)
	}
	if mode := os.Getenv("RUNNER_PARENT"); mode != "" {
		err := Run(os.Args[0], []string{"child"}, map[string]string{"SHARED": "vault"}, mode == "override")
		fmt.Fprintln(os.Stderr, "Run returned:", err)
		os.Exit(99)
	}
	os.Exit(m.Run())
}

func TestRun(t *testing.T) {
	for mode, want := range map[string]string{"override": "vault", "no-override": "host"} {
		cmd := exec.Command(os.Args[0])
		cmd.Env = append(os.Environ(), "RUNNER_PARENT="+mode, "SHARED=host")
		out, err := cmd.Output()

		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 {
			t.Fatalf("%s: expected child exit code 7, got %v", mode, err)
		}
		if string(out) != want {
			t.Errorf("%s: SHARED = %q, want %q", mode, out, want)
		}
	}

	if err := Run("envlock-no-such-command", nil, nil, true); err == nil {
		t.Fatal("expected error for missing command")
	}
}
