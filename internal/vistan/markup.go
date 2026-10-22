package vistan

import (
	"encoding/base64"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	bulletLine = regexp.MustCompile(`^[ \t]*[-*][ \t]`)
	indentOnly = regexp.MustCompile(`^[ \t]*`)
	taskLine   = regexp.MustCompile(`^[ \t]*[-*][ \t]+\[([ xX])\][ \t]+(.*)$`)
	fenceOpen  = regexp.MustCompile("^[ \\t]*```[ \\t]*([^` \\t]*)")
	calloutTag = regexp.MustCompile(`^\[!([A-Za-z]+)\][ \t]*(.*)$`)
	riskTag    = regexp.MustCompile(`(?i)^[ \t]*>[ \t]*\[!(warn|risk)\]`)
)
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
