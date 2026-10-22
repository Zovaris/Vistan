package vistan

import (
	"fmt"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type docEntry struct {
	name   string
	title  string
	kind   string
	status string
	when   string
	source string
	lines  []string
}

var statusRank = map[string]int{
	"accepted": 0, "approved": 1, "proposed": 2, "draft": 3,
	"rejected": 4, "superseded": 5, "deprecated": 6, "unknown": 9,
}

func (e docEntry) rank() int {
	if r, ok := statusRank[e.status]; ok {
		return r
	}
	return 8
}

func parseStatus(text string) (string, string) {
	text = strings.TrimSpace(strings.NewReplacer("**", "", "`", "").Replace(text))
	if text == "" {
		return "unknown", ""
	}
	fields := strings.Fields(text)
	status := strings.ToLower(strings.Trim(fields[0], ".,:;"))
	when := ""
	for _, sep := range []string{"—", "|", "·"} {
		if i := strings.Index(text, sep); i >= 0 {
			when = strings.TrimSpace(text[i+len(sep):])
			break
		}
	}
	if status == "" {
		status = "unknown"
	}
	return status, when
}

func collectDocs(root string) ([]docEntry, error) {
	var out []docEntry
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if path == root {
				return nil
			}
			if skipDirs[name] || name == "templates" || strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > 1<<20 {
			return nil
		}
		lines, err := readFileLines(path)
		if err != nil {
			return nil
		}

		doc := Parse(lines)
		kind := DeclaredTemplate(lines)
		if kind != "adr" && kind != "spec" {
			if doc.find("Status") == nil || doc.find("Decision") == nil {
				return nil
			}
			kind = "adr"
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		entry := docEntry{
			name:   pageName(strings.TrimSuffix(rel, ".md")),
			title:  doc.Title,
			kind:   kind,
			source: rel,
			lines:  lines,
		}
		if entry.title == "" {
			entry.title = strings.TrimSuffix(filepath.Base(rel), ".md")
		}
		status := ""
		if s := doc.find("Status"); s != nil {
			status = strings.Join(trimBlank(s.Body), " ")
		}
		entry.status, entry.when = parseStatus(status)
		if entry.when == "" {
			entry.when = info.ModTime().Format("2 Jan 2006")
		}
		out = append(out, entry)
		return nil
	})
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].rank() != out[j].rank() {
			return out[i].rank() < out[j].rank()
		}
		if out[i].when != out[j].when {
			return out[i].when > out[j].when
		}
		return out[i].title < out[j].title
	})
	return out, err
}

func readFileLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	if err := readLines(f, &lines); err != nil {
		return nil, err
	}
	return lines, nil
}

func runDocs(args []string) int {
	root := "."
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			fmt.Fprintf(os.Stderr, "unknown option: %s\n", arg)
			return 2
		}
		root = arg
	}

	entries, err := collectDocs(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if len(entries) == 0 {
		fmt.Fprintf(os.Stderr, "no ADRs or specs under %s; start one with vistan new adr\n", root)
		return 1
	}

	dir := stateDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	for _, entry := range entries {
		doc := Parse(entry.lines)
		tplName := entry.kind
		if _, ok := lookupTemplate(tplName); !ok {
			tplName = "adr"
		}
		tpl, _ := lookupTemplate(tplName)
		source := filepath.Join(root, entry.source)
		page := renderHTML(doc, tpl, source, filepath.Dir(source))
		if err := writePage(dir, entry.name, page, entry.lines); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "decisions.html"), []byte(decisionsHTML(entries)), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	refreshIndex(dir)

	base := serveBase(dir)
	fmt.Printf("%d record%s · %s  %sdecisions.html\n", len(entries), pluralSuffix(len(entries)),
		statusSummary(entries), base)
	for _, entry := range entries {
		fmt.Printf("  %-11s %-5s %-38s %s.html\n", entry.status, strings.ToUpper(entry.kind),
			clip(entry.title, 38), entry.name)
	}
	if base != "" && os.Getenv("VISTAN_NO_OPEN") == "" {
		openBrowser(base + "decisions.html")
	}
	if base == "" {
		return 2
	}
	return 0
}

func statusSummary(entries []docEntry) string {
	counts := map[string]int{}
	var order []string
	for _, entry := range entries {
		if counts[entry.status] == 0 {
			order = append(order, entry.status)
		}
		counts[entry.status]++
	}
	sort.SliceStable(order, func(i, j int) bool {
		ri, rj := statusRank[order[i]], statusRank[order[j]]
		if ri == 0 && order[i] != "accepted" {
			ri = 8
		}
		if rj == 0 && order[j] != "accepted" {
			rj = 8
		}
		return ri < rj
	})
	var parts []string
	for _, status := range order {
		parts = append(parts, fmt.Sprintf("%d %s", counts[status], status))
	}
	return strings.Join(parts, " · ")
}

func decisionsHTML(entries []docEntry) string {
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString(faviconTag)
	b.WriteString("<title>Decisions</title>\n<style>\n")
	b.WriteString(pageCSS)
	b.WriteString(decisionsCSS)
	b.WriteString("</style>\n</head>\n<body class=\"tpl-decisions\">\n<main>\n")
	b.WriteString("<a class=\"back\" href=\"/\">← History</a>\n")
	b.WriteString("<h1>Decisions</h1>\n")
	fmt.Fprintf(&b, "<p class=\"src\">%d record%s · %s</p>\n", len(entries), pluralSuffix(len(entries)), statusSummary(entries))

	var lastStatus string
	for _, entry := range entries {
		if entry.status != lastStatus {
			if lastStatus != "" {
				b.WriteString("</ul>\n")
			}
			count := 0
			for _, other := range entries {
				if other.status == entry.status {
					count++
				}
			}
			fmt.Fprintf(&b, "<h2 class=\"status is-%s\">%s <span>%d</span></h2>\n<ul class=\"records\">\n",
				sevSlug(entry.status), html.EscapeString(entry.status), count)
			lastStatus = entry.status
		}
		fmt.Fprintf(&b, "<li><a href=\"%s.html\"><span class=\"title\">%s</span></a>", entry.name, html.EscapeString(entry.title))
		fmt.Fprintf(&b, "<span class=\"kind\">%s</span><span class=\"when\">%s</span>", strings.ToUpper(entry.kind), html.EscapeString(entry.when))
		fmt.Fprintf(&b, "<span class=\"src-path\">%s</span></li>\n", html.EscapeString(entry.source))
	}
	if lastStatus != "" {
		b.WriteString("</ul>\n")
	}
	b.WriteString("<footer>Run <code>vistan docs</code> again after editing a record.</footer>\n</main>\n</body>\n</html>\n")
	return b.String()
}

const decisionsCSS = `.tpl-decisions main { width: min(52rem, calc(100vw - 2.5rem)); }
h2.status { display: flex; align-items: baseline; justify-content: space-between; gap: 1em;
  margin: 38px 0 0; padding-top: 16px; border-top: 1px solid var(--rule);
  font-size: 0.68rem; font-style: normal; letter-spacing: 0.09em; text-transform: uppercase;
  color: var(--muted); font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif; }
h2.status span { font-variant-numeric: tabular-nums; }
h2.status.is-accepted { color: #33623f; }
h2.status.is-proposed, h2.status.is-draft { color: #7a5620; }
h2.status.is-rejected { color: #8a3b2c; }
h2.status.is-superseded, h2.status.is-deprecated { color: var(--muted); }
ul.records { list-style: none; margin: 6px 0 0; padding: 0; }
.records li { display: flex; align-items: baseline; gap: 0.9em; padding: 10px 0;
  border-top: 1px solid var(--rule); }
.records li:first-child { border-top: 0; }
.records li a { flex: 1; min-width: 0; color: inherit; text-decoration: none; }
.records li a:hover .title { text-decoration: underline; text-underline-offset: 0.15em; }
.records .title { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.records .kind { flex: none; padding: 1px 7px; border: 1px solid var(--rule); border-radius: 999px;
  font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif; font-size: 0.58rem;
  letter-spacing: 0.08em; color: var(--muted); }
.records .when, .records .src-path { flex: none; color: var(--muted);
  font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif; font-size: 0.68rem;
  font-variant-numeric: tabular-nums; }
.records .src-path { width: 15em; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  text-align: right; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 0.65rem; }
@media (max-width: 640px) { .records .src-path { display: none; } }
`
