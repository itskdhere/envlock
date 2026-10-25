package env

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Paths holds the resolved file paths for the plaintext .env and ciphertext .env.lock.
type Paths struct {
	EnvFile  string
	LockFile string
}

// ResolvePaths determines the exact file paths based on flags.
// Priority:
// 1. Explicit fileFlag (--file / -f): EnvFile is fileFlag, LockFile is fileFlag + ".lock" (or replaces .env with .env.lock)
// 2. Named environment (--env / -e): e.g. "prod" -> .env.prod and .env.prod.lock
// 3. Default: .env and .env.lock in baseDir.
func ResolvePaths(baseDir string, envName string, fileFlag string) Paths {
	if fileFlag != "" {
		cleanFile := filepath.Clean(fileFlag)
		if !filepath.IsAbs(cleanFile) {
			cleanFile = filepath.Join(baseDir, cleanFile)
		}

		var lockFile string
		base := filepath.Base(cleanFile)
		dir := filepath.Dir(cleanFile)
		if base == ".env" {
			lockFile = filepath.Join(dir, ".env.lock")
		} else if strings.HasPrefix(base, ".env.") {
			lockFile = filepath.Join(dir, base+".lock")
		} else {
			lockFile = cleanFile + ".lock"
		}

		return Paths{
			EnvFile:  cleanFile,
			LockFile: lockFile,
		}
	}

	envName = strings.TrimSpace(envName)
	if envName != "" {
		envFileName := fmt.Sprintf(".env.%s", envName)
		lockFileName := fmt.Sprintf(".env.%s.lock", envName)
		return Paths{
			EnvFile:  filepath.Join(baseDir, envFileName),
			LockFile: filepath.Join(baseDir, lockFileName),
		}
	}

	return Paths{
		EnvFile:  filepath.Join(baseDir, ".env"),
		LockFile: filepath.Join(baseDir, ".env.lock"),
	}
}

