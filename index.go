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
	titleTag   = regexp.MustCompile(`<h1>(.*?)</h1>`)
	tplTag     = regexp.MustCompile(`<body class="tpl-([a-z]+)`)
	projectTag = regexp.MustCompile(`<meta name="vistan-project" content="([^"]*)"`)
)

type pageEntry struct {
	name    string
	title   string
	tpl     string
	project string
	mod     time.Time
}

func runIndex() int {
	dir := stateDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	var pages []pageEntry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".html") || entry.Name() == "index.html" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		page := pageEntry{name: entry.Name(), title: strings.TrimSuffix(entry.Name(), ".html"), mod: info.ModTime()}
		if b, err := os.ReadFile(filepath.Join(dir, entry.Name())); err == nil {
			if m := titleTag.FindSubmatch(b); m != nil {
				page.title = html.UnescapeString(string(m[1]))
			}
			if m := tplTag.FindSubmatch(b); m != nil {
				page.tpl = string(m[1])
			}
			if m := projectTag.FindSubmatch(b); m != nil {
				page.project = html.UnescapeString(string(m[1]))
			}
		}
		pages = append(pages, page)
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].mod.After(pages[j].mod) })

	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(indexHTML(pages)), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	port := 8765
	if p := os.Getenv("VISTAN_PORT"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}
	if _, err := startServer(dir, port); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if pid, known, ok := readPidFile(dir); ok && pid > 0 {
		port = known
	}
	base := fmt.Sprintf("http://127.0.0.1:%d/", port)
	fmt.Printf("%d page%s · %d project%s  %s\n", len(pages), pluralSuffix(len(pages)),
		projectCount(pages), pluralSuffix(projectCount(pages)), base)
	manyProjects := projectCount(pages) > 1
	for _, page := range pages {
		if manyProjects {
			fmt.Printf("  %s  %-9s %-12s %-30s %s%s\n", page.mod.Format("15:04"),
				page.tpl, clip(page.project, 12), clip(page.title, 30), base, page.name)
			continue
		}
		fmt.Printf("  %s  %-9s %-30s %s%s\n", page.mod.Format("15:04"),
			page.tpl, clip(page.title, 30), base, page.name)
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

func projectCount(pages []pageEntry) int {
	seen := map[string]bool{}
	for _, page := range pages {
		if page.project != "" {
			seen[page.project] = true
		}
	}
	return len(seen)
}

type dayGroup struct {
	label string
	pages []pageEntry
}

func groupByDay(pages []pageEntry) []dayGroup {
	var groups []dayGroup
	for _, page := range pages {
		label := dayLabel(page.mod)
		if len(groups) == 0 || groups[len(groups)-1].label != label {
			groups = append(groups, dayGroup{label: label})
		}
		groups[len(groups)-1].pages = append(groups[len(groups)-1].pages, page)
	}
	return groups
}

func dayLabel(t time.Time) string {
	t = t.Local()
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, now.Location())
	switch {
	case day.Equal(today):
		return "Today"
	case day.Equal(today.AddDate(0, 0, -1)):
		return "Yesterday"
	default:
		return day.Format("2 Jan 2006")
	}
}

func indexHTML(pages []pageEntry) string {
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString("<title>vistan history</title>\n<style>\n")
	b.WriteString(pageCSS)
	b.WriteString(indexCSS)
	b.WriteString("</style>\n</head>\n<body>\n<main>\n<h1>History</h1>\n")
	fmt.Fprintf(&b, "<p class=\"src\">%d page%s · %d project%s · newest first</p>\n",
		len(pages), pluralSuffix(len(pages)), projectCount(pages), pluralSuffix(projectCount(pages)))

	if len(pages) == 0 {
		b.WriteString("<p class=\"empty\">Nothing generated yet. Run <code>vistan serve file.md</code> or <code>vistan map .</code>.</p>\n")
	} else {
		manyProjects := projectCount(pages) > 1
		for _, group := range groupByDay(pages) {
			fmt.Fprintf(&b, "<h2 class=\"day\">%s</h2>\n<ul class=\"pages\">\n", html.EscapeString(group.label))
			for _, page := range group.pages {
				fmt.Fprintf(&b, "<li><a href=\"%s\"><span class=\"title\">%s</span></a><span class=\"tpl\">%s</span>",
					html.EscapeString(page.name), html.EscapeString(page.title), html.EscapeString(page.tpl))
				if manyProjects {
					fmt.Fprintf(&b, "<span class=\"proj\">%s</span>", html.EscapeString(page.project))
				}
				fmt.Fprintf(&b, "<span class=\"time\">%s</span></li>\n", page.mod.Format("15:04"))
			}
			b.WriteString("</ul>\n")
		}
	}

	keep := fmt.Sprintf("the last %d pages", keepLimit())
	if keepLimit() <= 0 {
		keep = "every page"
	} else if keepLimit() == 1 {
		keep = "the last page"
	}
	fmt.Fprintf(&b, "<footer>Keeping %s in <code>%s</code></footer>\n", keep, html.EscapeString(stateDir()))
	b.WriteString("</main>\n</body>\n</html>\n")
	return b.String()
}

const indexCSS = `h2.day { margin: 34px 0 0; font-size: 0.65rem; font-weight: 500; letter-spacing: 0.09em;
  text-transform: uppercase; color: var(--muted);
  font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif; }
ul.pages { list-style: none; margin: 6px 0 0; padding: 0; }
.pages li { display: flex; align-items: baseline; gap: 1em; padding: 10px 0;
  border-top: 1px solid var(--rule); }
.pages li a { flex: 1; min-width: 0; color: inherit; text-decoration: none; }
.pages li a:hover .title { text-decoration: underline; text-underline-offset: 0.15em; }
.pages .title { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pages span { flex: none; color: var(--muted); font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif;
  font-size: 0.7rem; font-variant-numeric: tabular-nums; }
.pages .tpl { width: 7em; letter-spacing: 0.07em; text-transform: uppercase; font-size: 0.62rem; }
.pages .proj { width: 8em; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pages .time { width: 3.2em; text-align: right; }
footer code { background: none; padding: 0; }
`
