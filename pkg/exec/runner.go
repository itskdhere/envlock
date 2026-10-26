package exec

import (
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
)

// Run replaces the current process with the command, layering vars over the host
// environment (vars win unless override is false). It returns only if the command
// could not be started.
func Run(name string, args []string, vars map[string]string, override bool) error {
	pairs := make([]string, 0, len(vars))
	for k, v := range vars {
		pairs = append(pairs, k+"="+v)
	}

	// os/exec keeps the last entry for duplicate keys (case-insensitively on Windows).
	var env []string
	if override {
		env = append(os.Environ(), pairs...)
	} else {
		env = append(pairs, os.Environ()...)
	}

	cmd := exec.Command(name, args...)
	if cmd.Err != nil {
		return cmd.Err
	}
	cmd.Env = env

	if runtime.GOOS != "windows" {
		// execve: signals and the exit code pass through natively, with no parent left to orphan the child.
		return syscall.Exec(cmd.Path, cmd.Args, cmd.Environ())
	}

	// Windows Ctrl+C reaches every process on the console; swallow it so we outlive the child and report its exit code.
	signal.Notify(make(chan os.Signal, 1), os.Interrupt)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	if cmd.ProcessState == nil {
		return err
	}
	os.Exit(cmd.ProcessState.ExitCode())
	return nil
}
