package vistan

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// removeState deletes the state directory, but never a path that holds more than
// vistan's pages: an empty VISTAN_CACHE, or one pointed at the wrong directory, must
// not turn a cleanup into a wiped home.
func removeState(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	abs = filepath.Clean(abs)
	if filepath.Dir(abs) == abs {
		return fmt.Errorf("refusing to delete %s: it is a filesystem root", abs)
	}
	if home, err := os.UserHomeDir(); err == nil && abs == filepath.Clean(home) {
		return fmt.Errorf("refusing to delete %s: it is the home directory", abs)
	}
	if wd, err := os.Getwd(); err == nil && abs == wd {
		return fmt.Errorf("refusing to delete %s: it is the working directory", abs)
	}
	if _, err := os.Stat(abs); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(abs)
}
func stateDir() string {
	if dir := os.Getenv("VISTAN_CACHE"); dir != "" {
		return dir
	}
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "vistan")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "vistan")
	}
	if runtime.GOOS == "windows" {
		if base, err := os.UserConfigDir(); err == nil {
			return filepath.Join(base, "vistan")
		}
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "vistan")
	}
	return filepath.Join(home, ".local", "state", "vistan")
}
func keepLimit() int {
	limit := 50
	if v := os.Getenv("VISTAN_KEEP"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	return limit
}
func keepDays() int {
	days := 30
	if v := os.Getenv("VISTAN_KEEP_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			days = n
		}
	}
	return days
}
func keepPages(dir string) {
	limit, days := keepLimit(), keepDays()
	if limit <= 0 && days <= 0 {
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	type page struct {
		name string
		mod  time.Time
	}
	var pages []page
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".html") || isIndexName(name) {
			continue
		}
		if info, err := entry.Info(); err == nil {
			pages = append(pages, page{name, info.ModTime()})
		}
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].mod.After(pages[j].mod) })
	cutoff := time.Now().AddDate(0, 0, -days)
	for i, p := range pages {
		if limit > 0 && i >= limit {
			removePage(dir, p.name)
			continue
		}
		if days > 0 && p.mod.Before(cutoff) {
			removePage(dir, p.name)
		}
	}
}
