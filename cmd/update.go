package cmd

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/itskdhere/envlock/pkg/ui"
	"github.com/mattn/go-isatty"
)

const (
	latestReleaseURL    = "https://github.com/itskdhere/envlock/releases/latest"
	updateCheckInterval = 6 * time.Hour
)

var (
	latestRelease chan string
	noticeOnce    sync.Once
)

func startUpdateCheck() {
	if _, ok := parseVersion(Version); !ok || os.Getenv("ENVLOCK_NO_UPDATE_CHECK") != "" || os.Getenv("CI") != "" {
		return
	}
	if fd := os.Stderr.Fd(); !isatty.IsTerminal(fd) && !isatty.IsCygwinTerminal(fd) {
		return
	}
	latestRelease = make(chan string, 1)
	go func() { latestRelease <- cachedLatestRelease() }()
}

func printUpdateNotice() {
	noticeOnce.Do(func() {
		if latestRelease == nil {
			return
		}
		latest := <-latestRelease
		if !newerVersion(latest, Version) {
			return
		}
		update := "curl -fsSL https://envlock.itskdhere.com | bash"
		if runtime.GOOS == "windows" {
			update = "irm https://envlock.itskdhere.com | iex"
		}
		fmt.Fprintln(os.Stderr, ui.Warning("EnvLock v%s is available (you have v%s). Update with: %s",
			strings.TrimPrefix(latest, "v"), strings.TrimPrefix(Version, "v"), update))
	})
}

func cachedLatestRelease() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	file := filepath.Join(dir, "envlock", "latest-release")
	cached, _ := os.ReadFile(file)
	if info, err := os.Stat(file); err == nil && time.Since(info.ModTime()) < updateCheckInterval {
		return string(cached)
	}
	latest, err := fetchLatestRelease()
	if err != nil {
		// Rewriting resets the mtime, so an offline machine doesn't retry on every run.
		latest = string(cached)
	}
	_ = os.MkdirAll(filepath.Dir(file), 0755)
	_ = os.WriteFile(file, []byte(latest), 0644)
	return latest
}

// The /releases/latest redirect has no rate limit, unlike the API.
func fetchLatestRelease() (string, error) {
	client := &http.Client{
		Timeout:       2 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Head(latestReleaseURL)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	location := resp.Header.Get("Location")
	tag := path.Base(location)
	if _, ok := parseVersion(tag); !ok {
		return "", fmt.Errorf("unexpected redirect to %q", location)
	}
	return tag, nil
}

// Strict X.Y.Z keeps dev builds from checking and untrusted text out of the terminal.
func parseVersion(v string) ([]uint64, bool) {
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return nil, false
	}
	nums := make([]uint64, len(parts))
	for i, p := range parts {
		n, err := strconv.ParseUint(p, 10, 32)
		if err != nil {
			return nil, false
		}
		nums[i] = n
	}
	return nums, true
}

func newerVersion(latest, current string) bool {
	l, okLatest := parseVersion(latest)
	c, okCurrent := parseVersion(current)
	return okLatest && okCurrent && slices.Compare(l, c) > 0
}
