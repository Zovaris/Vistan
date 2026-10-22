package vistan

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

func collectPages(dir string) ([]pageEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var pages []pageEntry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".html") || isIndexName(entry.Name()) {
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
	return pages, nil
}

func isIndexName(name string) bool {
	return name == "index.html" || name == "decisions.html"
}

func writeIndex(dir string, pages []pageEntry) error {
	return os.WriteFile(filepath.Join(dir, "index.html"), []byte(indexHTML(pages)), 0o644)
}

func refreshIndex(dir string) {
	pages, err := collectPages(dir)
	if err != nil {
		return
	}
	writeIndex(dir, pages)
}

func runIndex() int {
	dir := stateDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	pages, err := collectPages(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if err := writeIndex(dir, pages); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	base := serveBase(dir)
	fmt.Printf("%d page%s · %d project%s  %s\n", len(pages), pluralSuffix(len(pages)),
		projectCount(pages), pluralSuffix(projectCount(pages)), base)
	for _, group := range groupByProject(pages) {
		fmt.Printf("%s  %d page%s · last %s\n", group.name, len(group.pages),
			pluralSuffix(len(group.pages)), lastSeen(group.last))
		for _, page := range group.pages {
			fmt.Printf("  %s  %-9s %-34s %s%s\n", page.mod.Format("15:04"),
				page.tpl, clip(page.title, 34), base, page.name)
		}
	}
	if os.Getenv("VISTAN_NO_OPEN") == "" {
		openBrowser(base)
	}
	return 0
}

func resolvePage(dir, query string) (string, []string) {
	pages, err := collectPages(dir)
	if err != nil {
		return "", nil
	}
	q := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(query, ".html")))
	if q == "" {
		return "", nil
	}

	var exact, prefix, loose []string
	for _, page := range pages {
		slug := strings.ToLower(strings.TrimSuffix(page.name, ".html"))
		switch {
		case slug == q:
			exact = append(exact, page.name)
		case strings.HasPrefix(slug, q):
			prefix = append(prefix, page.name)
		case strings.Contains(slug, q), strings.Contains(strings.ToLower(page.title), q):
			loose = append(loose, page.name)
		}
	}

	for _, matches := range [][]string{exact, prefix, loose} {
		if len(matches) == 1 {
			return filepath.Join(dir, matches[0]), nil
		}
		if len(matches) > 1 {
			return "", matches
		}
	}
	return "", nil
}

func runOpen(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: vistan open <name>")
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

	base := serveBase(dir)
	if base == "" {
		return 2
	}
	url := base + filepath.Base(path)
	fmt.Println(url)
	if os.Getenv("VISTAN_NO_OPEN") == "" {
		openBrowser(url)
	}
	return 0
}

func runRm(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: vistan rm <name> [name...]")
		return 2
	}
	dir := stateDir()
	code := 0
	removed := 0
	for _, query := range args {
		path, candidates := resolvePage(dir, query)
		if path == "" {
			if len(candidates) > 0 {
				fmt.Fprintf(os.Stderr, "ambiguous: %s\n", strings.Join(candidates, " "))
			} else {
				fmt.Fprintf(os.Stderr, "no page matching %s\n", query)
			}
			code = 2
			continue
		}
		if err := os.Remove(path); err != nil {
			fmt.Fprintln(os.Stderr, err)
			code = 2
			continue
		}
		fmt.Printf("removed %s\n", filepath.Base(path))
		removed++
	}

	if removed > 0 {
		refreshIndex(dir)
	}
	return code
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
		seen[page.project] = true
	}
	return len(seen)
}

func projectLabel(name string) string {
	if name == "" {
		return "unfiled"
	}
	return name
}

func lastSeen(t time.Time) string {
	t = t.Local()
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return "today " + t.Format("15:04")
	}
	if t.Year() == now.Year() {
		return t.Format("2 Jan")
	}
	return t.Format("2 Jan 2006")
}

type projectGroup struct {
	name  string
	pages []pageEntry
	last  time.Time
}

func groupByProject(pages []pageEntry) []projectGroup {
	var groups []projectGroup
	index := map[string]int{}
	for _, page := range pages {
		label := projectLabel(page.project)
		i, ok := index[label]
		if !ok {
			groups = append(groups, projectGroup{name: label, last: page.mod})
			i = len(groups) - 1
			index[label] = i
			groups[i].pages = append(groups[i].pages, page)
			continue
		}
		groups[i].pages = append(groups[i].pages, page)
	}
	sort.Slice(groups, func(i, j int) bool {
		if !groups[i].last.Equal(groups[j].last) {
			return groups[i].last.After(groups[j].last)
		}
		return groups[i].name < groups[j].name
	})
	return groups
}

func activityStrip(pages []pageEntry) string {
	const span = 30
	counts := map[string]int{}
	for _, page := range pages {
		counts[page.mod.Local().Format("2006-01-02")]++
	}

	now := time.Now()
	var b strings.Builder
	b.WriteString(`<div class="heat" role="img" aria-label="Pages generated per day over the last 30 days">` + "\n")
	for i := span - 1; i >= 0; i-- {
		day := now.AddDate(0, 0, -i)
		n := counts[day.Format("2006-01-02")]
		level := n
		if level > 4 {
			level = 4
		}
		fmt.Fprintf(&b, `<span class="cell l%d" title="%s · %d page%s"></span>`+"\n",
			level, day.Format("2 Jan"), n, pluralSuffix(n))
	}
	b.WriteString("</div>\n")
	return b.String()
}

func indexHTML(pages []pageEntry) string {
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString(faviconTag)
	b.WriteString("<title>vistan history</title>\n<style>\n")
	b.WriteString(pageCSS)
	b.WriteString(indexCSS)
	b.WriteString("</style>\n</head>\n<body>\n<main>\n<h1>History</h1>\n")
	fmt.Fprintf(&b, "<p class=\"src\">%d page%s · %d project%s · newest first</p>\n",
		len(pages), pluralSuffix(len(pages)), projectCount(pages), pluralSuffix(projectCount(pages)))

	if len(pages) == 0 {
		b.WriteString("<p class=\"empty\">Nothing generated yet. Run <code>vistan serve file.md</code> or <code>vistan map .</code>.</p>\n")
	} else {
		now := time.Now()
		today := 0
		for _, page := range pages {
			if stamp := page.mod.Local(); stamp.Year() == now.Year() && stamp.YearDay() == now.YearDay() {
				today++
			}
		}
		fmt.Fprintf(&b, "<div class=\"heat-cap\"><span>Last 30 days · %s → today</span><span>%d today</span></div>\n",
			now.AddDate(0, 0, -29).Format("2 Jan"), today)
		b.WriteString(activityStrip(pages))
		for _, group := range groupByProject(pages) {
			fmt.Fprintf(&b, "<h2 class=\"proj\">%s <span>%d page%s · last %s</span></h2>\n<ul class=\"pages\">\n",
				html.EscapeString(group.name), len(group.pages), pluralSuffix(len(group.pages)), lastSeen(group.last))
			for _, page := range group.pages {
				fmt.Fprintf(&b, "<li><a href=\"%s\"><span class=\"title\">%s</span></a><span class=\"tpl\">%s</span>",
					html.EscapeString(page.name), html.EscapeString(page.title), html.EscapeString(page.tpl))
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

const indexCSS = `.heat-cap { display: flex; justify-content: space-between; gap: 1em; margin: 34px 0 6px;
  font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif; font-size: 0.6rem;
  letter-spacing: 0.08em; text-transform: uppercase; color: var(--muted); }
.heat { display: grid; grid-template-columns: repeat(30, 1fr); gap: 3px; margin-top: 8px; }
.heat .cell { aspect-ratio: 1; border-radius: 2px; background: var(--ink); opacity: 0.05; }
.heat .cell:hover { outline: 1px solid var(--ink); outline-offset: 1px; opacity: 1; }
.heat .l1 { opacity: 0.25; }
.heat .l2 { opacity: 0.45; }
.heat .l3 { opacity: 0.65; }
.heat .l4 { opacity: 0.9; }
h2.proj { display: flex; align-items: baseline; justify-content: space-between; gap: 1em;
  margin: 38px 0 0; padding-top: 16px; border-top: 1px solid var(--rule);
  font-size: 1rem; font-style: normal; font-weight: 400; }
h2.proj span { font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif;
  font-size: 0.68rem; letter-spacing: 0.05em; color: var(--muted);
  font-variant-numeric: tabular-nums; }
ul.pages { list-style: none; margin: 4px 0 0; padding: 0; }
.pages li { display: flex; align-items: baseline; gap: 1em; padding: 9px 0;
  border-top: 1px solid var(--rule); }
.pages li:first-child { border-top: 0; }
.pages li a { flex: 1; min-width: 0; color: inherit; text-decoration: none; }
.pages li a:hover .title { text-decoration: underline; text-underline-offset: 0.15em; }
.pages .title { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pages span { flex: none; color: var(--muted); font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif;
  font-size: 0.7rem; font-variant-numeric: tabular-nums; }
.pages .tpl { width: 7em; letter-spacing: 0.07em; text-transform: uppercase; font-size: 0.62rem; }
.pages .time { width: 3.2em; text-align: right; }
footer code { background: none; padding: 0; }
`
