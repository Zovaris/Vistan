package vistan

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

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
<p class="hint" id="hint">Tick the findings you want applied.</p>
<div class="answer">
<p class="choice" id="ans" aria-live="polite">Reject all findings</p>
<button type="button" id="send">Send decision</button>
</div>
<script>
const boxes = Array.from(document.querySelectorAll('input.pick'));
const ans = document.getElementById('ans');
const send = document.getElementById('send');
const hint = document.getElementById('hint');
function sync() {
  const picked = boxes.filter(b => b.checked).map(b => b.value);
  ans.textContent = picked.length ? 'Apply ' + picked.join(', ') : 'Reject all findings';
}
boxes.forEach(b => b.addEventListener('change', sync));
document.querySelectorAll('section.finding').forEach(card => {
  card.addEventListener('click', e => {
    if (e.target.closest('input, a, button')) return;
    if (window.getSelection().toString()) return;
    const box = card.querySelector('input.pick');
    box.checked = !box.checked;
    box.dispatchEvent(new Event('change', { bubbles: true }));
  });
});
send.addEventListener('click', () => sendAnswer(ans.textContent, send, hint));
</script>
</section>
`

// sendAnswer posts the choice back to the server that served the page, so the agent
// can read it with `vistan answer <page>` instead of the human pasting it in a chat.
const sendAnswer = `<script>
async function sendAnswer(value, button, hint) {
  const name = location.pathname.split('/').pop().replace(/\.html$/, '');
  button.disabled = true;
  hint.textContent = 'Sending…';
  try {
    const res = await fetch('/_answer/' + encodeURIComponent(name), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ answer: value }),
    });
    if (!res.ok) throw new Error(res.status);
    button.closest('.answer').classList.add('sent');
    hint.textContent = 'Sent. The agent reads it when you go back to the chat.';
  } catch (e) {
    button.disabled = false;
    hint.textContent = 'Could not reach the server. Run vistan serve again on this page.';
  }
}
</script>
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
<p class="choice none" id="ans" aria-live="polite">No option picked yet</p>
<button type="button" id="send" disabled>Send choice</button>
</div>
<p class="hint" id="hint">Pick an option above.</p>
<script>
const ans = document.getElementById('ans');
const send = document.getElementById('send');
const hint = document.getElementById('hint');
function pick(r) {
  ans.textContent = r.value;
  ans.classList.remove('none');
  send.disabled = false;
}
for (const r of document.querySelectorAll('input[name=opt]')) {
  r.addEventListener('change', () => pick(r));
}
document.querySelectorAll('section.opt').forEach(card => {
  const radio = card.querySelector('input[name=opt]');
  if (!radio) return;
  card.addEventListener('click', e => {
    if (e.target.closest('input, a, button')) return;
    if (window.getSelection().toString()) return;
    radio.checked = true;
    radio.dispatchEvent(new Event('change', { bubbles: true }));
  });
});
send.addEventListener('click', () => sendAnswer(ans.textContent, send, hint));
</script>
`

func absDir(dir string) string {
	if dir == "" {
		return ""
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return abs
}

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
	fmt.Fprintf(&b, "<meta name=\"vistan-source\" content=\"%s\">\n", html.EscapeString(source))
	if base := absDir(baseDir); base != "" {
		fmt.Fprintf(&b, "<meta name=\"vistan-base\" content=\"%s\">\n", html.EscapeString(base))
	}
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
		b.WriteString(sendAnswer)
	}

	if selection {
		b.WriteString("<section class=\"opt\">\n<label class=\"pick\"><input type=\"radio\" name=\"opt\" value=\"None: ask for another option\"><span class=\"key\">-</span><span class=\"ttl\">None of these</span></label>\n</section>\n")
		b.WriteString("</fieldset>\n")
		b.WriteString(selectionScript)
		b.WriteString(sendAnswer)
	} else if missing := d.Missing(t.Required); len(missing) > 0 {
		fmt.Fprintf(&b, "<footer><span class=\"warn\">Missing or empty: %s</span></footer>\n", html.EscapeString(strings.Join(missing, " ")))
	} else {
		fmt.Fprintf(&b, "<footer>%s</footer>\n", html.EscapeString(t.Label))
	}

	b.WriteString("</main>\n</body>\n</html>\n")
	return b.String()
}
