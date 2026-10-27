//go:build !windows

package env

import "os"

func restrictToOwner(path string) error {
	return os.Chmod(path, FilePermOwnerOnly)
}
