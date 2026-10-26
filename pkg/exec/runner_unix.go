//go:build !windows

package exec

import (
	"os/exec"
	"syscall"
)

// execve, so signals and the exit code pass through natively.
func run(cmd *exec.Cmd) error {
	return syscall.Exec(cmd.Path, cmd.Args, cmd.Environ())
}
