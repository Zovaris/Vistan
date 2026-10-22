package main

import (
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	ansiReset = "\033[0m"
	ansiBold  = "\033[1m"
	ansiDim   = "\033[2m"
	ansiCyan  = "\033[1;36m"
	ansiMag   = "\033[1;35m"
	ansiGreen = "\033[1;32m"
	ansiYell  = "\033[1;33m"
	ansiWhite = "\033[1;37m"
	ansiOK    = "\033[32m"
	ansiWarn  = "\033[33m"
)

var sectionColors = map[string]string{
	"Decisions":     ansiCyan,
	"Rejected":      ansiMag,
	"Verification":  ansiGreen,
	"Your move":     ansiYell,
	"What changed":  ansiCyan,
	"Pending":       ansiYell,
	"Overview":      ansiCyan,
	"Tree":          ansiGreen,
	"Entry points":  ansiMag,
	"Largest files": ansiYell,
	"Status":        ansiMag,
	"Context":       ansiCyan,
	"Decision":      ansiGreen,
	"Consequences":  ansiYell,
	"Added":         ansiGreen,
	"Changed":       ansiCyan,
	"Fixed":         ansiYell,
	"Removed":       ansiMag,
}

var (
	bulletLine = regexp.MustCompile(`^[ \t]*[-*][ \t]`)
	indentOnly = regexp.MustCompile(`^[ \t]*`)
	taskLine   = regexp.MustCompile(`^[ \t]*[-*][ \t]+\[([ xX])\][ \t]+(.*)$`)
	fenceOpen  = regexp.MustCompile("^[ \\t]*```[ \\t]*([^` \\t]*)")
	calloutTag = regexp.MustCompile(`^\[!([A-Za-z]+)\][ \t]*(.*)$`)
	riskTag    = regexp.MustCompile(`(?i)^[ \t]*>[ \t]*\[!(warn|risk)\]`)
)

var calloutLabels = map[string]string{
	"note": "Note", "tip": "Tip", "warn": "Warning", "risk": "Risk",
	"decision": "Decision", "question": "Open question", "done": "Done",
}

func renderTerminal(d *Doc, t Template, source string, color bool, w io.Writer) int {
	paint := func(code string) string {
		if !color {
			return ""
		}
		return code
	}

	fmt.Fprintf(w, "\n  %s%s%s\n", paint(ansiBold), d.Title, paint(ansiReset))
	fmt.Fprintf(w, "  %s%s%s\n", paint(ansiDim), source, paint(ansiReset))

	if intro := trimBlank(d.Intro); len(intro) > 0 {
		fmt.Fprintln(w)
		for _, line := range intro {
			if fenceLine.MatchString(line) {
				continue
			}
			fmt.Fprintf(w, "  %s\n", line)
		}
	}

	selection := t.Name == "selection"
	for i, s := range d.Sections {
		body := s.BodyTrimmed()
		color := paint(sectionColors[s.Name])
		if color == "" && paint(ansiWhite) != "" {
			color = paint(ansiWhite)
		}

		prefix := ""
		if selection {
			prefix = fmt.Sprintf("%c) ", 'A'+rune(i))
		}

		fmt.Fprintf(w, "\n  %s▌%s %s%s%s%s", color, paint(ansiReset), color, prefix, s.Name, paint(ansiReset))
		items := countChanges(body)
		if items == 0 {
			for _, line := range body {
				if bulletLine.MatchString(line) {
					items++
				}
			}
		}
		switch {
		case len(body) == 0:
			fmt.Fprintf(w, " %s· empty%s\n", paint(ansiDim), paint(ansiReset))
			continue
		case items > 0:
			fmt.Fprintf(w, " %s· %d%s\n", paint(ansiDim), items, paint(ansiReset))
		default:
			fmt.Fprint(w, "\n")
		}

		for _, line := range body {
			trimmed := strings.TrimSpace(line)
			switch {
			case fenceLine.MatchString(line):
				continue
			case strings.HasPrefix(trimmed, "### "):
				fmt.Fprintf(w, "    %s%s%s\n", paint(ansiDim), strings.TrimSpace(trimmed[4:]), paint(ansiReset))
			case strings.HasPrefix(trimmed, ">"):
				text := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
				if m := calloutTag.FindStringSubmatch(text); m != nil {
					label := calloutLabels[strings.ToLower(m[1])]
					if label == "" {
						label = m[1]
					}
					fmt.Fprintf(w, "  %s! %s%s", paint(ansiWarn), strings.ToLower(label), paint(ansiReset))
					if m[2] != "" {
						fmt.Fprintf(w, " — %s", m[2])
					}
					fmt.Fprintln(w)
					continue
				}
				fmt.Fprintf(w, "  %s%s%s\n", paint(ansiDim), text, paint(ansiReset))
			case taskLine.MatchString(line):
				m := taskLine.FindStringSubmatch(line)
				box := "[ ]"
				if m[1] != " " {
					box = "[x]"
				}
				fmt.Fprintf(w, "  %s %s\n", box, m[2])
			case imageLine.MatchString(line):
				m := imageLine.FindStringSubmatch(line)
				label := m[1]
				if label == "" {
					label = m[2]
				}
				fmt.Fprintf(w, "  %s▣ %s%s\n", paint(ansiDim), label, paint(ansiReset))
			default:
				fmt.Fprintf(w, "  %s\n", line)
			}
		}
	}

	if missing := d.Missing(t.Required); len(missing) > 0 {
		fmt.Fprintf(w, "\n  %s! missing or empty: %s%s\n", paint(ansiWarn), strings.Join(missing, " "), paint(ansiReset))
		return 1
	}
	fmt.Fprintf(w, "\n  %s+ %s%s\n", paint(ansiOK), strings.ToLower(t.Label), paint(ansiReset))
	return 0
}

var (
	inlineToken = regexp.MustCompile("`([^`]+)`|\\*\\*([^*]+)\\*\\*|\\[([^\\]]+)\\]\\(([^)\\s]+)\\)")
	imageLine   = regexp.MustCompile(`^!\[([^\]]*)\]\(([^)\s]+)\)\s*$`)
)

func inline(s string) string { return inlineEscaped(html.EscapeString(s)) }

func inlineEscaped(s string) string {
	matches := inlineToken.FindAllStringSubmatchIndex(s, -1)
	if len(matches) == 0 {
		return s
	}

	var b strings.Builder
	last := 0
	for _, m := range matches {
		b.WriteString(s[last:m[0]])
		switch {
		case m[2] >= 0:
			b.WriteString("<code>" + s[m[2]:m[3]] + "</code>")
		case m[4] >= 0:
			b.WriteString("<strong>" + inlineEscaped(s[m[4]:m[5]]) + "</strong>")
		case m[6] >= 0:
			href := s[m[8]:m[9]]
			if safeURL(html.UnescapeString(href)) {
				b.WriteString(`<a href="` + href + `">` + s[m[6]:m[7]] + `</a>`)
			} else {
				b.WriteString(s[m[0]:m[1]])
			}
		}
		last = m[1]
	}
	b.WriteString(s[last:])
	return b.String()
}

func safeURL(u string) bool {
	lower := strings.ToLower(u)
	for _, prefix := range []string{"http://", "https://", "mailto:", "#", "/", "./", "../"} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return !strings.Contains(u, ":")
}

var imageMimes = map[string]string{
	"png": "image/png", "jpg": "image/jpeg", "jpeg": "image/jpeg",
	"gif": "image/gif", "webp": "image/webp", "avif": "image/avif", "svg": "image/svg+xml",
}

func imageSrc(src, baseDir string) string {
	lower := strings.ToLower(src)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "data:") {
		return html.EscapeString(src)
	}
	if baseDir == "" {
		return html.EscapeString(src)
	}
	b, err := os.ReadFile(filepath.Join(baseDir, src))
	if err != nil || len(b) > 3<<20 {
		return html.EscapeString(src)
	}
	mime, ok := imageMimes[strings.ToLower(strings.TrimPrefix(filepath.Ext(src), "."))]
	if !ok {
		return html.EscapeString(src)
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b)
}

func renderBody(b *strings.Builder, lines []string, baseDir string) {
	inList := false
	inFence := false
	fenceKind := ""

	closeList := func() {
		if inList {
			b.WriteString("</ul>\n")
			inList = false
		}
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		switch {
		case inFence:
			if fenceLine.MatchString(line) {
				if fenceKind == "changes" {
					b.WriteString("</ul>\n")
					inList = false
				} else {
					b.WriteString("</code></pre>\n")
				}
				inFence = false
				fenceKind = ""
				continue
			}
			if fenceKind == "changes" {
				renderChange(b, line)
				continue
			}
			b.WriteString(html.EscapeString(line))
			b.WriteString("\n")
		case fenceOpen.MatchString(line):
			closeList()
			fenceKind = fenceOpen.FindStringSubmatch(line)[1]
			inFence = true
			if fenceKind == "changes" {
				b.WriteString("<ul class=\"changes\">\n")
				inList = true
				continue
			}
			b.WriteString("<pre><code>\n")
		case strings.HasPrefix(trimmed, ">"):
			closeList()
			var quote []string
			for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ">") {
				quote = append(quote, strings.TrimPrefix(strings.TrimSpace(lines[i]), ">"))
				i++
			}
			i--
			renderCallout(b, quote)
		case strings.HasPrefix(trimmed, "### "):
			closeList()
			fmt.Fprintf(b, "<h3>%s</h3>\n", inline(strings.TrimSpace(trimmed[4:])))
		case taskLine.MatchString(line):
			if !inList {
				b.WriteString("<ul>\n")
				inList = true
			}
			m := taskLine.FindStringSubmatch(line)
			done := m[1] != " "
			class, box := "task", "&#9744;"
			if done {
				class, box = "task done", "&#10003;"
			}
			level := len(indentOnly.FindString(line)) / 2
			fmt.Fprintf(b, "<li class=\"%s\" style=\"--l:%d\"><span class=\"box\">%s</span><span class=\"txt\">%s</span></li>\n",
				class, level, box, inline(m[2]))
		case bulletLine.MatchString(line):
			if !inList {
				b.WriteString("<ul>\n")
				inList = true
			}
			level := len(indentOnly.FindString(line)) / 2
			text := strings.TrimSpace(bulletLine.ReplaceAllString(line, ""))
			fmt.Fprintf(b, "<li style=\"--l:%d\">%s</li>\n", level, inline(text))
		case imageLine.MatchString(line):
			closeList()
			m := imageLine.FindStringSubmatch(line)
			fmt.Fprintf(b, "<figure><img src=\"%s\" alt=\"%s\">", imageSrc(m[2], baseDir), html.EscapeString(m[1]))
			if m[1] != "" {
				fmt.Fprintf(b, "<figcaption>%s</figcaption>", html.EscapeString(m[1]))
			}
			b.WriteString("</figure>\n")
		case trimmed == "":
			closeList()
		default:
			closeList()
			fmt.Fprintf(b, "<p>%s</p>\n", inline(line))
		}
	}
	if inFence {
		if fenceKind == "changes" {
			b.WriteString("</ul>\n")
		} else {
			b.WriteString("</code></pre>\n")
		}
	}
	closeList()
}

func renderChange(b *strings.Builder, line string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return
	}

	var kind, letter string
	switch trimmed[0] {
	case '+':
		kind, letter = "add", "A"
	case '~':
		kind, letter = "mod", "M"
	case '-':
		kind, letter = "del", "D"
	case '>':
		kind, letter = "move", "R"
	default:
		fmt.Fprintf(b, "<li><code>%s</code></li>\n", html.EscapeString(trimmed))
		return
	}

	path, note := strings.TrimSpace(trimmed[1:]), ""
	if i := strings.Index(path, "  "); i > 0 {
		path, note = strings.TrimSpace(path[:i]), strings.TrimSpace(path[i:])
	}

	fmt.Fprintf(b, `<li class="%s"><span class="mark">%s</span><code>%s</code>`, kind, letter, html.EscapeString(path))
	if note != "" {
		fmt.Fprintf(b, `<span class="note">— %s</span>`, inline(note))
	}
	b.WriteString("</li>\n")
}

func renderCallout(b *strings.Builder, lines []string) {
	kind, title := "note", ""
	if len(lines) > 0 {
		if m := calloutTag.FindStringSubmatch(strings.TrimSpace(lines[0])); m != nil {
			kind = strings.ToLower(m[1])
			title = strings.TrimSpace(m[2])
			lines = lines[1:]
		}
	}
	label := calloutLabels[kind]
	if label == "" {
		label = strings.ToUpper(kind[:1]) + kind[1:]
	}

	fmt.Fprintf(b, `<aside class="callout %s"><p class="kind">%s</p>`, kind, html.EscapeString(label))
	if title != "" {
		fmt.Fprintf(b, `<p class="lead">%s</p>`, inline(title))
	}

	var para []string
	inList := false
	flush := func() {
		if len(para) > 0 {
			fmt.Fprintf(b, "<p>%s</p>\n", inline(strings.Join(para, " ")))
			para = nil
		}
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			flush()
			if inList {
				b.WriteString("</ul>\n")
				inList = false
			}
		case bulletLine.MatchString(line):
			flush()
			if !inList {
				b.WriteString("<ul>\n")
				inList = true
			}
			fmt.Fprintf(b, "<li>%s</li>\n", inline(strings.TrimSpace(bulletLine.ReplaceAllString(line, ""))))
		default:
			para = append(para, trimmed)
		}
	}
	flush()
	if inList {
		b.WriteString("</ul>\n")
	}
	b.WriteString("</aside>\n")
}

func countChanges(lines []string) int {
	count, in := 0, false
	for _, line := range lines {
		if fenceLine.MatchString(line) {
			in = !in && strings.TrimSpace(fenceOpen.FindStringSubmatch(line)[1]) == "changes"
			continue
		}
		if in && strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func statStrip(d *Doc) string {
	tasks, done, files, risks := 0, 0, 0, 0
	for _, s := range d.Sections {
		for _, line := range s.Body {
			if m := taskLine.FindStringSubmatch(line); m != nil {
				tasks++
				if m[1] != " " {
					done++
				}
			}
			if riskTag.MatchString(line) {
				risks++
			}
		}
		files += countChanges(s.Body)
	}

	sevCount := map[string]int{}
	findings := 0
	for _, s := range d.Sections {
		if _, severity, _, ok := parseFinding(s.Name); ok {
			findings++
			sevCount[sevSlug(severity)]++
		}
	}

	var parts []string
	if findings > 0 {
		parts = append(parts, fmt.Sprintf("<span><b>%d</b> finding%s</span>", findings, pluralSuffix(findings)))
		for _, severity := range []string{"blocker", "critical", "major", "important", "minor", "nit", "question", "pre-existing"} {
			if n := sevCount[severity]; n > 0 {
				label := severity
				if n > 1 && severity != "pre-existing" {
					label += "s"
				}
				parts = append(parts, fmt.Sprintf("<span><b>%d</b> %s</span>", n, label))
			}
		}
	}
	if tasks > 0 {
		parts = append(parts, fmt.Sprintf("<span><b>%d/%d</b> checks</span>", done, tasks))
	}
	if files > 0 {
		parts = append(parts, fmt.Sprintf("<span><b>%d</b> file%s</span>", files, pluralSuffix(files)))
	}
	if risks > 0 {
		parts = append(parts, fmt.Sprintf("<span><b>%d</b> risk%s</span>", risks, pluralSuffix(risks)))
	}
	if move := d.find("Your move"); move != nil && len(move.BodyTrimmed()) > 0 {
		parts = append(parts, `<span class="wait">waiting on you</span>`)
	}
	if len(parts) == 0 {
		return ""
	}
	return `<p class="stats">` + strings.Join(parts, "") + "</p>\n"
}

func pluralSuffix(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func parseFinding(name string) (id, severity, location string, ok bool) {
	parts := strings.Split(name, "·")
	if len(parts) < 2 {
		return "", "", "", false
	}
	id = strings.TrimSpace(parts[0])
	severity = strings.TrimSpace(parts[1])
	if len(parts) > 2 {
		location = strings.TrimSpace(strings.Join(parts[2:], " · "))
	}
	location = strings.Trim(strings.TrimSpace(location), "`")
	if id == "" || severity == "" || strings.ContainsAny(id, " \t") {
		return "", "", "", false
	}
	return id, severity, location, true
}

func sevSlug(severity string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(severity) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func renderFinding(b *strings.Builder, id, severity, location string, body []string, baseDir string) {
	anchor := html.EscapeString(strings.ToLower(sevSlug(id)))
	fmt.Fprintf(b, `<section class="finding sev-%s" id="%s">`+"\n", sevSlug(severity), anchor)
	fmt.Fprintf(b, `<div class="fhead"><label class="apply"><input type="checkbox" class="pick" value="%s" aria-label="Apply %s"><span>%s</span></label>`,
		html.EscapeString(id), html.EscapeString(id), html.EscapeString(id))
	fmt.Fprintf(b, `<span class="sev">%s</span>`, html.EscapeString(severity))
	if location != "" {
		fmt.Fprintf(b, `<code class="loc">%s</code>`, html.EscapeString(location))
	}
	b.WriteString("</div>\n<div class=\"fbody\">\n")
	if len(body) == 0 {
		b.WriteString(`<p class="empty">no detail</p>`)
	} else {
		renderBody(b, body, baseDir)
	}
	b.WriteString("</div>\n</section>\n")
}

const reviewDecision = `<section class="decision">
<div class="dhead"><p class="hint">Tick the findings you want applied, then copy the decision.</p></div>
<div class="answer">
<input id="ans" readonly value="Reject all findings" aria-label="Decision to copy">
<button type="button" id="copy">Copy</button>
</div>
<script>
const boxes = Array.from(document.querySelectorAll('input.pick'));
const ans = document.getElementById('ans');
const copy = document.getElementById('copy');
function sync() {
  const picked = boxes.filter(b => b.checked).map(b => b.value);
  ans.value = picked.length ? 'Apply ' + picked.join(', ') : 'Reject all findings';
}
boxes.forEach(b => b.addEventListener('change', sync));
copy.addEventListener('click', async () => {
  try {
    await navigator.clipboard.writeText(ans.value);
    copy.textContent = 'Copied';
    setTimeout(() => { copy.textContent = 'Copy'; }, 1400);
  } catch (e) {
    ans.select();
    copy.textContent = 'Cmd+C';
  }
});
</script>
</section>
`

func statusClass(body []string) string {
	text := strings.ToLower(strings.Join(body, " "))
	for _, status := range []string{"accepted", "proposed", "rejected", "superseded", "deprecated"} {
		if strings.Contains(text, status) {
			return "is-" + status
		}
	}
	return "is-unknown"
}

func unreleasedClass(name string) string {
	if strings.Contains(strings.ToLower(name), "unreleased") {
		return " is-unreleased"
	}
	return ""
}

const selectionScript = `<div class="answer">
<input id="ans" readonly placeholder="Pick an option" aria-label="Answer to copy">
<button type="button" id="copy" disabled>Copy</button>
</div>
<p class="hint" id="hint">Pick an option, then copy the answer into the chat.</p>
<script>
const ans = document.getElementById('ans');
const copy = document.getElementById('copy');
const hint = document.getElementById('hint');
for (const r of document.querySelectorAll('input[name=opt]')) {
  r.addEventListener('change', () => {
    ans.value = r.value;
    copy.disabled = false;
    hint.textContent = 'Copy the answer and paste it in the chat.';
  });
}
copy.addEventListener('click', async () => {
  try {
    await navigator.clipboard.writeText(ans.value);
    hint.textContent = 'Copied: ' + ans.value;
  } catch (e) {
    ans.select();
    hint.textContent = 'Copy failed. It is selected: use Cmd+C.';
  }
});
</script>
`

func projectName(baseDir string) string {
	dir := baseDir
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return ""
		}
		dir = wd
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return filepath.Base(dir)
	}
	for root := abs; ; {
		if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
			return filepath.Base(root)
		}
		parent := filepath.Dir(root)
		if parent == root {
			return filepath.Base(abs)
		}
		root = parent
	}
}

const faviconTag = `<link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' width='32' height='32' viewBox='0 0 32 32'><rect width='32' height='32' rx='7' fill='%23f6f3ea'/><text x='16' y='23' font-family='Georgia,serif' font-size='20' text-anchor='middle' fill='%231b1a17'>v</text></svg>">` + "\n"

func renderHTML(d *Doc, t Template, source, baseDir string) string {
	var b strings.Builder

	title := d.Title
	if title == "" {
		title = "document"
	}

	b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString(faviconTag)
	fmt.Fprintf(&b, "<title>%s</title>\n", html.EscapeString(title))
	fmt.Fprintf(&b, "<meta name=\"vistan-project\" content=\"%s\">\n", html.EscapeString(projectName(baseDir)))
	b.WriteString("<style>\n" + pageCSS + "</style>\n</head>\n<body class=\"tpl-" + t.Name + "\">\n<main>\n")
	b.WriteString("<a class=\"back\" href=\"/\">← History</a>\n")
	fmt.Fprintf(&b, "<h1>%s</h1>\n", html.EscapeString(title))
	fmt.Fprintf(&b, "<p class=\"src\">%s</p>\n", html.EscapeString(source))

	renderBody(&b, trimBlank(d.Intro), baseDir)
	if d.Artifact != "" {
		b.WriteString("<div class=\"art\">")
		b.WriteString(d.Artifact)
		b.WriteString("</div>\n")
	}

	selection := t.Name == "selection"
	review := t.Name == "review"
	graph := t.Name == "graph"
	if selection {
		b.WriteString("<fieldset class=\"opts\">\n<legend class=\"sr-only\">Pick one</legend>\n")
	} else {
		b.WriteString(statStrip(d))
	}
	findings := 0

	for i, s := range d.Sections {
		body := s.BodyTrimmed()
		if selection {
			key := string(rune('A' + i))
			value := key + ": " + strings.NewReplacer("`", "", "*", "").Replace(s.Name)
			fmt.Fprintf(&b, "<section class=\"opt\">\n<label class=\"pick\"><input type=\"radio\" name=\"opt\" value=\"%s\"><span class=\"key\">%s</span><span class=\"ttl\">%s</span></label>\n<div class=\"desc\">\n",
				html.EscapeString(value), key, inline(s.Name))
			if len(body) == 0 {
				b.WriteString("<p class=\"empty\">no description</p>\n")
			} else {
				renderBody(&b, body, baseDir)
			}
			b.WriteString("</div>\n</section>\n")
			continue
		}

		if review {
			if id, severity, location, ok := parseFinding(s.Name); ok {
				renderFinding(&b, id, severity, location, body, baseDir)
				findings++
				continue
			}
		}

		if graph && strings.EqualFold(s.Name, "Graph") {
			if svg := graphSVG(body); svg != "" {
				b.WriteString("<section class=\"drawing\">\n<div class=\"art\">")
				b.WriteString(svg)
				b.WriteString("</div>\n</section>\n")
				continue
			}
		}

		items, tasks, done := 0, 0, 0
		for _, line := range body {
			if m := taskLine.FindStringSubmatch(line); m != nil {
				tasks++
				if m[1] != " " {
					done++
				}
				continue
			}
			if bulletLine.MatchString(line) {
				items++
			}
		}
		changes := countChanges(body)

		class := ""
		switch {
		case t.Name == "adr" && i == 0:
			class = ` class="status ` + statusClass(body) + `"`
		case t.Name == "adr":
			class = ` class="record"`
		case t.Name == "changelog":
			class = ` class="release` + unreleasedClass(s.Name) + `"`
		case t.Name == "plan" && s.Name == "Your move":
			class = ` class="move"`
		}
		fmt.Fprintf(&b, "<section%s>\n", class)

		label := ""
		switch {
		case t.Name == "changelog", t.Name == "plan" && s.Name == "Your move":
		case tasks > 0:
			label = fmt.Sprintf("%d/%d", done, tasks)
		case changes > 0:
			label = strconv.Itoa(changes)
		case items > 0:
			label = strconv.Itoa(items)
		}
		if label != "" {
			fmt.Fprintf(&b, "<h2>%s <span>%s</span></h2>\n", inline(s.Name), label)
		} else {
			fmt.Fprintf(&b, "<h2>%s</h2>\n", inline(s.Name))
		}
		if len(body) == 0 {
			b.WriteString("<p class=\"empty\">no content</p>\n")
		} else {
			renderBody(&b, body, baseDir)
		}
		b.WriteString("</section>\n")
	}

	if review && findings > 0 {
		b.WriteString(reviewDecision)
	}

	if selection {
		b.WriteString("<section class=\"opt\">\n<label class=\"pick\"><input type=\"radio\" name=\"opt\" value=\"None: ask for another option\"><span class=\"key\">-</span><span class=\"ttl\">None of these</span></label>\n</section>\n")
		b.WriteString("</fieldset>\n")
		b.WriteString(selectionScript)
	} else if missing := d.Missing(t.Required); len(missing) > 0 {
		fmt.Fprintf(&b, "<footer><span class=\"warn\">Missing or empty: %s</span></footer>\n", html.EscapeString(strings.Join(missing, " ")))
	} else {
		fmt.Fprintf(&b, "<footer>%s</footer>\n", html.EscapeString(t.Label))
	}

	b.WriteString("</main>\n</body>\n</html>\n")
	return b.String()
}

const pageCSS = `:root {
  color-scheme: light dark;
  --paper: #f6f3ea;
  --ink: #1b1a17;
  --muted: #625e55;
  --rule: #d2ccbf;
  --field: #fffdf6;
}
* { box-sizing: border-box; }
html { background: var(--paper); }
body { margin: 0; background: var(--paper); color: var(--ink);
  font-family: "Iowan Old Style", "Palatino Linotype", "URW Palladio L", P052, Palatino, Georgia, serif;
  font-size: 17px; line-height: 1.6; -webkit-font-smoothing: antialiased; }
main { width: min(38rem, calc(100vw - 2.5rem)); margin: 0 auto; padding: 80px 0 96px; }
h1 { margin: 0 0 8px; font-size: 2.05rem; line-height: 1.16; font-weight: 400;
  letter-spacing: -0.015em; text-wrap: balance; }
.src { margin: 0; color: var(--muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 0.72rem; }
section { margin-top: 32px; padding-top: 20px; border-top: 1px solid var(--rule); }
h2 { display: flex; align-items: baseline; justify-content: space-between; gap: 1em;
  margin: 0 0 10px; font-size: 1.05rem; font-style: italic; font-weight: 400; }
h2 span { font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif;
  font-size: 0.7rem; font-style: normal; letter-spacing: 0.05em; color: var(--muted);
  font-variant-numeric: tabular-nums; }
ul { margin: 4px 0 0; padding-left: 1.2em; }
li { margin: 0 0 8px; margin-left: calc(var(--l, 0) * 1.2em); }
li:last-child { margin-bottom: 0; }
li::marker { color: var(--muted); }
p { margin: 12px 0; }
code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 0.85em;
  background: #ece5d5; border-radius: 2px; padding: 0.1em 0.3em; }
pre { margin: 14px 0; padding: 14px 16px; overflow-x: auto; background: #efe9da;
  border: 1px solid var(--rule); border-radius: 3px; }
pre code { background: none; padding: 0; font-size: 0.8rem; line-height: 1.5; }
.empty { margin: 0; color: var(--muted); font-style: italic; }

.stats { display: flex; flex-wrap: wrap; gap: 1.3em; margin: 26px 0 0;
  font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif;
  font-size: 0.73rem; letter-spacing: 0.02em; color: var(--muted); }
.stats b { color: var(--ink); font-weight: 500; font-variant-numeric: tabular-nums; }
.stats .wait { color: var(--ink); border-bottom: 1px solid var(--rule); padding-bottom: 1px; }

li.task { list-style: none; position: relative; padding-left: 1.5em;
  margin-left: calc(var(--l, 0) * 1.2em - 1.15em); }
li.task .box { position: absolute; left: 0; top: 0.28em; width: 1.02em; height: 1.02em;
  display: inline-flex; align-items: center; justify-content: center;
  border: 1px solid var(--rule); border-radius: 3px; font-size: 0.66em;
  color: transparent; font-family: ui-sans-serif, sans-serif; }
li.task.done .box { background: var(--ink); border-color: var(--ink); color: var(--paper); }
li.task.done .txt { color: var(--muted); text-decoration: line-through;
  text-decoration-color: var(--rule); }

ul.changes { list-style: none; margin: 12px 0; padding: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 0.8rem; }
ul.changes li { display: flex; align-items: baseline; gap: 0.6em; margin: 0 0 5px;
  margin-left: 0; }
ul.changes .mark { flex: none; width: 1.15em; text-align: center; font-size: 0.68rem;
  font-weight: 600; color: var(--muted); }
ul.changes .add .mark { color: #3f7a4d; }
ul.changes .del .mark { color: #a1503f; }
ul.changes .move .mark { color: #5d6f8c; }
ul.changes code { background: none; padding: 0; white-space: pre-wrap; word-break: break-word; }
ul.changes .note { color: var(--muted); font-family: inherit; font-size: 0.78rem; }

aside.callout { margin: 18px 0; padding: 12px 16px 14px; border-left: 2px solid var(--rule);
  background: #efe9da; border-radius: 0 3px 3px 0; }
aside.callout .kind { margin: 0 0 5px; font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif;
  font-size: 0.62rem; letter-spacing: 0.1em; text-transform: uppercase; color: var(--muted); }
aside.callout .lead { margin: 0 0 4px; font-weight: 600; }
aside.callout p { margin: 6px 0; }
aside.callout p:last-child, aside.callout ul:last-child { margin-bottom: 0; }
aside.callout ul { margin: 4px 0 0; }
aside.callout.tip { background: #e9f0e4; border-left-color: #8ca57e; }
aside.callout.warn, aside.callout.risk { background: #f6e9dd; border-left-color: #c08a5e; }
aside.callout.decision { background: #e8ecf3; border-left-color: #8496b4; }
aside.callout.question { background: #f3eef7; border-left-color: #a58ab8; }

.tpl-review .finding { margin-top: 22px; padding: 16px 0 2px 16px; border-left: 2px solid var(--rule); }
.tpl-review .fhead { display: flex; align-items: baseline; flex-wrap: wrap; gap: 0.7em; }
.tpl-review .apply { display: flex; align-items: center; gap: 0.45em; cursor: pointer;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 0.78rem; color: var(--muted); }
.tpl-review .apply input { margin: 0; accent-color: var(--ink); }
.tpl-review .sev { font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif;
  font-size: 0.61rem; letter-spacing: 0.08em; text-transform: uppercase; padding: 1px 8px;
  border: 1px solid var(--rule); border-radius: 999px; color: var(--muted); }
.tpl-review .loc { margin-left: auto; background: none; padding: 0; font-size: 0.74rem; color: var(--muted); }
.tpl-review .fbody { margin-top: 8px; }
.tpl-review .fbody > :first-child { margin-top: 0; }
.tpl-review .sev-blocker, .tpl-review .sev-critical { border-left-color: #b4553c; }
.tpl-review .sev-blocker .sev, .tpl-review .sev-critical .sev { color: #8a3b2c; border-color: #dcc0b6; background: #f6e9e2; }
.tpl-review .sev-major, .tpl-review .sev-important { border-left-color: #c08a5e; }
.tpl-review .sev-major .sev, .tpl-review .sev-important .sev { color: #7a5620; border-color: #ddcbaa; background: #f6efdf; }
.tpl-review .sev-minor { border-left-color: #8496b4; }
.tpl-review .sev-minor .sev { color: #46586f; border-color: #c3ccdb; background: #eaeef4; }
.tpl-review .sev-question { border-left-color: #a58ab8; }
.tpl-review .sev-question .sev { color: #5d4470; border-color: #d3c3dd; background: #f2ecf6; }
.tpl-review section.decision { margin-top: 42px; padding: 18px 24px 20px; border: 1px solid var(--rule);
  border-radius: 4px; background: var(--field); }
.tpl-review section.decision .hint { margin: 0 0 12px; }
.tpl-review section.decision::before { content: "your decision"; display: block; margin-bottom: 8px;
  font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif; font-size: 0.62rem;
  letter-spacing: 0.09em; text-transform: uppercase; color: var(--muted); }

.tpl-plan section.move { margin-top: 42px; padding: 20px 24px 22px; border: 1px solid var(--rule);
  border-radius: 4px; background: var(--field); }
.tpl-plan section.move h2::after { content: "waiting on you"; font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif;
  font-size: 0.62rem; font-style: normal; letter-spacing: 0.09em; text-transform: uppercase;
  color: var(--muted); }

footer { margin-top: 40px; padding-top: 16px; border-top: 1px solid var(--rule);
  font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif;
  font-size: 0.75rem; letter-spacing: 0.02em; color: var(--muted); }
footer .warn { color: #8a4b1f; }

fieldset.opts { border: 0; padding: 0; margin: 36px 0 0; }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px;
  overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; border: 0; }
.opt .pick { display: flex; align-items: baseline; gap: 0.7rem; cursor: pointer; }
.opt .pick input { margin: 0; accent-color: var(--ink); }
.opt .key { font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.78rem; color: var(--muted); min-width: 1.1em; }
.opt .ttl { font-size: 1.05rem; }
.opt .desc { margin: 6px 0 0 2.1rem; }
.opt .desc > :first-child { margin-top: 0; }
.opt .desc > :last-child { margin-bottom: 0; }
.opt:has(input:checked) { border-top-color: var(--ink); }
.answer { display: flex; gap: 10px; margin-top: 30px; align-items: stretch; }
.answer input { flex: 1; min-width: 0; font: inherit; font-size: 0.9rem; padding: 8px 11px;
  border: 1px solid var(--rule); border-radius: 3px; background: var(--field); color: var(--ink); }
.answer button { font: inherit; font-size: 0.85rem; padding: 8px 15px; cursor: pointer;
  border: 1px solid var(--ink); border-radius: 3px; background: var(--ink); color: var(--paper); }
.answer button:disabled { opacity: 0.35; cursor: default; }
.hint { margin: 10px 0 0; color: var(--muted); font-size: 0.78rem;
  font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif; }

h3 { margin: 20px 0 6px; font-size: 0.72rem; font-weight: 500; letter-spacing: 0.08em;
  text-transform: uppercase; color: var(--muted);
  font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif; }
h3:first-child { margin-top: 4px; }
.art { margin: 22px 0 8px; }
.art svg { display: block; width: 100%; height: auto; }
.art text { font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  fill: var(--ink); fill-opacity: 0.78; }
.art text.sub { fill-opacity: 0.42; }
.art .tm rect { stroke: var(--paper); stroke-width: 1.5; }
.art .tm-c0 rect { fill: #b4713d; }
.art .tm-c1 rect { fill: #7f8f56; }
.art .tm-c2 rect { fill: #6d7f9c; }
.art .tm-c3 rect { fill: #a76a80; }
.art .tm-c4 rect { fill: #b39445; }
.art .tm-c5 rect { fill: #5e8c85; }
.art .tm-d0 rect { fill-opacity: 0.3; }
.art .tm-d1 rect { fill-opacity: 0.16; }
figure { margin: 16px 0; }
figure img { display: block; max-width: 100%; height: auto;
  border: 1px solid var(--rule); border-radius: 3px; }
figcaption { margin-top: 6px; color: var(--muted); font-size: 0.75rem;
  font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif; }
.opt .desc figure { margin: 12px 0; }
.tpl-diagram main { width: min(46rem, calc(100vw - 2.5rem)); }
.tpl-diagram pre { font-size: 0.92rem; padding: 22px 24px; line-height: 1.35;
  background-color: transparent; border-radius: 4px;
  background-image: radial-gradient(circle, var(--rule) 1px, transparent 1px);
  background-size: 16px 16px; background-position: 8px 8px; }
.tpl-diagram section:first-of-type pre { padding: 30px 28px; }

.back { display: inline-block; margin-bottom: 16px; text-decoration: none;
  font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif; font-size: 0.62rem;
  letter-spacing: 0.09em; text-transform: uppercase; color: var(--muted); }
.back:hover { color: var(--ink); }

.tpl-graph main { width: min(52rem, calc(100vw - 2.5rem)); }
.tpl-graph section.drawing { border-top: 0; padding-top: 8px; margin-top: 12px; }
.art .graph .node rect { fill: var(--field); stroke: var(--rule); stroke-width: 1; }
.art .graph .node text { font-size: 11px; fill: var(--ink); fill-opacity: 0.85; }
.art .graph .edge { fill: none; stroke: var(--muted); stroke-width: 1.1; stroke-opacity: 0.55; }
.art .graph .edge.back { stroke-dasharray: 4 4; }
.art .graph .arrowhead { fill: var(--muted); fill-opacity: 0.7; }
.art .graph .edge-label { font-size: 9px; fill: var(--muted); fill-opacity: 0.8; }

.tpl-adr section.status { border-top: 0; margin-top: 14px; padding-top: 0; }
.tpl-adr section.status h2 { display: none; }
.tpl-adr section.status p { display: inline-block; margin: 0; padding: 2px 11px;
  border: 1px solid var(--rule); border-radius: 999px; font-size: 0.78rem; color: var(--muted);
  font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif; }
.tpl-adr section.status strong { font-weight: 500; }
.tpl-adr section.status.is-accepted p { color: #33623f; border-color: #b6cdb9; background: #eaf1e6; }
.tpl-adr section.status.is-rejected p { color: #8a3b2c; border-color: #dcc0b6; background: #f6e9e2; }
.tpl-adr section.status.is-proposed p { color: #7a5620; border-color: #ddcbaa; background: #f6efdf; }
.tpl-adr section.status.is-superseded p,
.tpl-adr section.status.is-deprecated p { text-decoration: line-through; }
.tpl-adr .record h2 { font-style: normal; }

.tpl-changelog h2 { font-style: normal; font-size: 1.15rem;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: -0.01em; }
.tpl-changelog .release.is-unreleased h2 { color: var(--muted); }
.tpl-changelog ul { margin-top: 0; }

@media (max-width: 560px) { main { padding: 48px 0 64px; } h1 { font-size: 1.7rem; } }
@media (prefers-color-scheme: dark) {
  :root { --paper: #13120e; --ink: #e7e1d3; --muted: #a49c88; --rule: #2a2720; --field: #1a1813; }
  .art .tm-d0 rect { fill-opacity: 0.5; }
  .art .tm-d1 rect { fill-opacity: 0.3; }
  aside.callout { background: #1c1a14; }
  aside.callout.tip { background: #171e16; }
  aside.callout.warn, aside.callout.risk { background: #221a14; }
  aside.callout.decision { background: #171b22; }
  aside.callout.question { background: #1d1822; }
  .tpl-review .sev-blocker .sev, .tpl-review .sev-critical .sev { color: #d8a295; border-color: #4a302a; background: #261a17; }
  .tpl-review .sev-major .sev, .tpl-review .sev-important .sev { color: #d6bd8a; border-color: #453a24; background: #221f16; }
  .tpl-review .sev-minor .sev { color: #a9bcd6; border-color: #2c3644; background: #171c24; }
  .tpl-review .sev-question .sev { color: #c3aed4; border-color: #3a2f44; background: #1f1a25; }
  ul.changes .add .mark { color: #8fbd95; }
  ul.changes .del .mark { color: #d29a88; }
  ul.changes .move .mark { color: #9aaecb; }
  .tpl-adr section.status.is-accepted p { color: #a8c8a8; border-color: #33452f; background: #1b241a; }
  .tpl-adr section.status.is-rejected p { color: #d8a295; border-color: #4a302a; background: #261a17; }
  .tpl-adr section.status.is-proposed p { color: #d6bd8a; border-color: #453a24; background: #221f16; }
  code { background: #221f18; }
  pre { background: #191712; }
  footer .warn { color: #d09a6a; }
  .answer button { background: var(--ink); color: #13120e; }
}
`
