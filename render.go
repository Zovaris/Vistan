package main

import (
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"regexp"
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
)

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
		items := 0
		for _, line := range body {
			if bulletLine.MatchString(line) {
				items++
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
			if fenceLine.MatchString(line) {
				continue
			}
			fmt.Fprintf(w, "  %s\n", line)
		}
	}

	if missing := d.Missing(t.Required); len(missing) > 0 {
		fmt.Fprintf(w, "\n  %s! missing or empty: %s%s\n", paint(ansiWarn), strings.Join(missing, " "), paint(ansiReset))
		return 1
	}
	fmt.Fprintf(w, "\n  %s+ %s%s\n", paint(ansiOK), strings.ToLower(t.Label), paint(ansiReset))
	return 0
}

func inline(s string) string {
	s = html.EscapeString(s)
	if !strings.Contains(s, "`") {
		return emphasis(s)
	}
	parts := strings.Split(s, "`")
	var b strings.Builder
	for i, part := range parts {
		if i%2 == 1 {
			b.WriteString("<code>" + part + "</code>")
			continue
		}
		b.WriteString(emphasis(part))
	}
	if len(parts)%2 == 0 {
		b.WriteString("</code>")
	}
	return b.String()
}

var (
	boldText  = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	linkText  = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	imageLine = regexp.MustCompile(`^!\[([^\]]*)\]\(([^)\s]+)\)\s*$`)
)

func emphasis(s string) string {
	s = boldText.ReplaceAllString(s, "<strong>$1</strong>")
	s = linkText.ReplaceAllStringFunc(s, func(m string) string {
		groups := linkText.FindStringSubmatch(m)
		if !safeURL(groups[2]) {
			return m
		}
		return `<a href="` + groups[2] + `">` + groups[1] + `</a>`
	})
	return s
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

	closeList := func() {
		if inList {
			b.WriteString("</ul>\n")
			inList = false
		}
	}

	for _, line := range lines {
		switch {
		case fenceLine.MatchString(line):
			if inFence {
				b.WriteString("</code></pre>\n")
				inFence = false
				continue
			}
			closeList()
			b.WriteString("<pre><code>\n")
			inFence = true
		case inFence:
			b.WriteString(html.EscapeString(line))
			b.WriteString("\n")
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
		case strings.TrimSpace(line) == "":
			closeList()
		default:
			closeList()
			fmt.Fprintf(b, "<p>%s</p>\n", inline(line))
		}
	}
	if inFence {
		b.WriteString("</code></pre>\n")
	}
	closeList()
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

func renderHTML(d *Doc, t Template, source, baseDir string) string {
	var b strings.Builder

	title := d.Title
	if title == "" {
		title = "document"
	}

	b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	fmt.Fprintf(&b, "<title>%s</title>\n", html.EscapeString(title))
	b.WriteString("<style>\n" + pageCSS + "</style>\n</head>\n<body class=\"tpl-" + t.Name + "\">\n<main>\n")
	fmt.Fprintf(&b, "<h1>%s</h1>\n", html.EscapeString(title))
	fmt.Fprintf(&b, "<p class=\"src\">%s</p>\n", html.EscapeString(source))

	renderBody(&b, trimBlank(d.Intro), baseDir)

	selection := t.Name == "selection"
	if selection {
		b.WriteString("<fieldset class=\"opts\">\n<legend class=\"sr-only\">Pick one</legend>\n")
	}

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

		items := 0
		for _, line := range body {
			if bulletLine.MatchString(line) {
				items++
			}
		}
		b.WriteString("<section>\n")
		if items > 0 {
			fmt.Fprintf(&b, "<h2>%s <span>%d</span></h2>\n", inline(s.Name), items)
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

figure { margin: 16px 0; }
figure img { display: block; max-width: 100%; height: auto;
  border: 1px solid var(--rule); border-radius: 3px; }
figcaption { margin-top: 6px; color: var(--muted); font-size: 0.75rem;
  font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif; }
.tpl-diagram main { width: min(46rem, calc(100vw - 2.5rem)); }
.tpl-diagram pre { font-size: 0.92rem; padding: 22px 24px; }

.tpl-adr section:first-of-type { border-top: 0; margin-top: 14px; padding-top: 0; }
.tpl-adr section:first-of-type h2 { display: none; }
.tpl-adr section:first-of-type p { display: inline-block; margin: 0; padding: 2px 11px;
  border: 1px solid var(--rule); border-radius: 999px; font-size: 0.78rem; color: var(--muted);
  font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif; }
.tpl-adr section:first-of-type strong { font-weight: 500; }

@media (max-width: 560px) { main { padding: 48px 0 64px; } h1 { font-size: 1.7rem; } }
@media (prefers-color-scheme: dark) {
  :root { --paper: #13120e; --ink: #e7e1d3; --muted: #a49c88; --rule: #2a2720; --field: #1a1813; }
  code { background: #221f18; }
  pre { background: #191712; }
  footer .warn { color: #d09a6a; }
  .answer button { background: var(--ink); color: #13120e; }
}
`
