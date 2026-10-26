package exec

import (
	"os"
	"os/exec"
)

// Run replaces the current process with the command; it returns only if the command could not be started.
func Run(name string, args []string, vars map[string]string, override bool) error {
	pairs := make([]string, 0, len(vars))
	for k, v := range vars {
		pairs = append(pairs, k+"="+v)
	}

	// os/exec keeps the last of duplicate keys, so order decides precedence.
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
	return run(cmd)
}
