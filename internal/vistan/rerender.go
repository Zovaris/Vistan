package vistan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func rerenderHTML(dir, name string) (string, []string, error) {
	lines, err := readFileLines(filepath.Join(dir, name+".md"))
	if err != nil {
		return "", nil, err
	}
	tplName := DeclaredTemplate(lines)
	if tplName == "" {
		tplName = "plan"
	}
	tpl, ok := lookupTemplate(tplName)
	if !ok {
		return "", nil, fmt.Errorf("unknown template: %s", tplName)
	}
	source, baseDir := pageMeta(dir, name)
	return renderHTML(Parse(lines), tpl, source, baseDir), lines, nil
}
func rerenderPage(dir, name, path string) error {
	page, lines, err := rerenderHTML(dir, name)
	if err != nil {
		return err
	}
	stamp := time.Now()
	if info, err := os.Stat(path); err == nil {
		stamp = info.ModTime()
	}
	if err := writePage(dir, name, page, lines); err != nil {
		return err
	}
	// The stamp only orders the history; a page that keeps its own mtime is still served.
	_ = os.Chtimes(filepath.Join(dir, name+".html"), stamp, stamp)
	_ = os.Chtimes(filepath.Join(dir, name+".md"), stamp, stamp)
	return nil
}
func runRerender(args []string) int {
	if len(args) == 1 && args[0] == "--all" {
		return rerenderAll()
	}
	if len(args) == 0 || strings.HasPrefix(args[0], "--") {
		fmt.Fprintln(os.Stderr, "usage: vistan rerender <name> | rerender --all")
		return 2
	}
	dir := stateDir()
	path, candidates := resolvePage(dir, args[0])
	if path == "" {
		if len(candidates) > 0 {
			fmt.Fprintf(os.Stderr, "ambiguous: %s\n", strings.Join(candidates, " "))
			return 2
		}
		fmt.Fprintf(os.Stderr, "no page matching %s; run vistan history\n", args[0])
		return 2
	}

	name := strings.TrimSuffix(filepath.Base(path), ".html")
	if err := rerenderPage(dir, name, path); err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "%s has no stored source; map pages cannot be re-rendered\n", name)
			return 2
		}
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	keepPages(dir)
	refreshIndex(dir)

	base := serveBase(dir)
	if base == "" {
		return 2
	}
	url := base + name + ".html"
	fmt.Println(url)
	if os.Getenv("VISTAN_NO_OPEN") == "" {
		openBrowser(url)
	}
	return 0
}
func rerenderAll() int {
	dir := stateDir()
	pages, err := collectPages(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	done, skipped, failed := 0, 0, 0
	for _, page := range pages {
		name := strings.TrimSuffix(page.name, ".html")
		if page.source == "" {
			skipped++
			continue
		}
		if err := rerenderPage(dir, name, filepath.Join(dir, page.name)); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
			failed++
			continue
		}
		done++
	}
	refreshIndex(dir)

	fmt.Printf("re-rendered %d of %d page%s", done, len(pages), pluralSuffix(len(pages)))
	if skipped > 0 {
		fmt.Printf(", %d with no stored source", skipped)
	}
	if failed > 0 {
		fmt.Printf(", %d failed", failed)
	}
	fmt.Println()
	if failed > 0 {
		return 2
	}
	return 0
}
