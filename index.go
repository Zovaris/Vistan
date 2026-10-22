package main

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	titleTag = regexp.MustCompile(`<h1>(.*?)</h1>`)
	tplTag   = regexp.MustCompile(`<body class="tpl-([a-z]+)`)
)

type pageEntry struct {
	name  string
	title string
	tpl   string
	mod   time.Time
}

func runIndex() int {
	cache := cacheDir()
	if err := os.MkdirAll(cache, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	dir, err := os.ReadDir(cache)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	var pages []pageEntry
	for _, entry := range dir {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".html") || entry.Name() == "index.html" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		page := pageEntry{name: entry.Name(), title: strings.TrimSuffix(entry.Name(), ".html"), mod: info.ModTime()}
		if b, err := os.ReadFile(filepath.Join(cache, entry.Name())); err == nil {
			if m := titleTag.FindSubmatch(b); m != nil {
				page.title = html.UnescapeString(string(m[1]))
			}
			if m := tplTag.FindSubmatch(b); m != nil {
				page.tpl = string(m[1])
			}
		}
		pages = append(pages, page)
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].mod.After(pages[j].mod) })

	if err := os.WriteFile(filepath.Join(cache, "index.html"), []byte(indexHTML(pages)), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	port := 8765
	if p := os.Getenv("VISTAN_PORT"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}
	if _, err := startServer(cache, port); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if pid, known, ok := readPidFile(cache); ok && pid > 0 {
		port = known
	}
	base := fmt.Sprintf("http://127.0.0.1:%d/", port)
	fmt.Printf("%d pages  %s\n", len(pages), base)
	for _, page := range pages {
		fmt.Printf("  %-34s %-9s %s%s\n", clip(page.title, 34), page.tpl, base, page.name)
	}
	if os.Getenv("VISTAN_NO_OPEN") == "" {
		openBrowser(base)
	}
	return 0
}

func clip(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}

func indexHTML(pages []pageEntry) string {
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString("<title>vistan pages</title>\n<style>\n")
	b.WriteString(pageCSS)
	b.WriteString(indexCSS)
	b.WriteString("</style>\n</head>\n<body>\n<main>\n<h1>Pages</h1>\n")
	fmt.Fprintf(&b, "<p class=\"src\">%d generated</p>\n", len(pages))

	if len(pages) == 0 {
		b.WriteString("<p class=\"empty\">Nothing generated yet. Run <code>vistan serve file.md</code> or <code>vistan map .</code>.</p>\n")
	} else {
		b.WriteString("<ul class=\"pages\">\n")
		for _, page := range pages {
			fmt.Fprintf(&b, "<li><a href=\"%s\">%s</a><span>%s</span><span>%s</span></li>\n",
				html.EscapeString(page.name), html.EscapeString(page.title), html.EscapeString(page.tpl), page.mod.Format("15:04"))
		}
		b.WriteString("</ul>\n")
	}
	b.WriteString("</main>\n</body>\n</html>\n")
	return b.String()
}

const indexCSS = `.pages { list-style: none; margin: 32px 0 0; padding: 0; }
.pages li { display: flex; align-items: baseline; gap: 1em;
  padding: 12px 0; border-top: 1px solid var(--rule); }
.pages li a { flex: 1; }
.pages a { color: inherit; text-decoration: none; }
.pages a:hover { text-decoration: underline; text-underline-offset: 0.15em; }
.pages span { color: var(--muted); font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif;
  font-size: 0.72rem; font-variant-numeric: tabular-nums; }
.pages li span:first-of-type { letter-spacing: 0.08em; text-transform: uppercase; font-size: 0.64rem; }
.pages li span:empty { display: none; }
`
