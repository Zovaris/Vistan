package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func parse(t *testing.T, text string) *Doc {
	t.Helper()
	return Parse(strings.Split(text, "\n"))
}

func tpl(t *testing.T, name string) Template {
	t.Helper()
	template, ok := lookupTemplate(name)
	if !ok {
		t.Fatalf("template %q not found", name)
	}
	return template
}

const planDoc = `# A plan

Intro line.

## Decisions

- one
- two

## Rejected

- three

## Verification

- ran it

## Your move

- nothing
`

func TestParseSections(t *testing.T) {
	d := parse(t, planDoc)
	if d.Title != "A plan" {
		t.Fatalf("title = %q", d.Title)
	}
	if len(d.Sections) != 4 {
		t.Fatalf("sections = %d, want 4", len(d.Sections))
	}
	if got := len(d.Sections[0].BodyTrimmed()); got != 2 {
		t.Fatalf("decisions body = %d lines, want 2", got)
	}
	if intro := trimBlank(d.Intro); len(intro) != 1 || intro[0] != "Intro line." {
		t.Fatalf("intro = %q", intro)
	}

	withMarker := parse(t, "<!-- vistan: plan -->\n\n"+planDoc)
	if intro := trimBlank(withMarker.Intro); len(intro) != 1 {
		t.Fatalf("marker leaked into intro: %q", intro)
	}
}

func TestFenceHidesHeadings(t *testing.T) {
	d := parse(t, "# T\n\n## A\n\n```\n## not a section\n```\n\n- x\n")
	if len(d.Sections) != 1 {
		t.Fatalf("sections = %d, want 1", len(d.Sections))
	}
	if name := d.Sections[0].Name; name != "A" {
		t.Fatalf("section = %q", name)
	}
}

func TestMissing(t *testing.T) {
	d := parse(t, "# T\n\n## Decisions\n- one\n")
	missing := d.Missing(tpl(t, "plan").Required)
	if len(missing) != 3 {
		t.Fatalf("missing = %v", missing)
	}
	d2 := parse(t, "# T\n\n## Decisions\n\n## Rejected\n- x\n\n## Verification\n- x\n\n## Your move\n- x\n")
	empty := d2.Missing(tpl(t, "plan").Required)
	if len(empty) != 1 || empty[0] != "Decisions" {
		t.Fatalf("empty section = %v, want [Decisions]", empty)
	}
}

func TestHTMLPlanHasSectionsAndFooter(t *testing.T) {
	html := renderHTML(parse(t, planDoc), tpl(t, "plan"), "examples/plan.md", "")
	if got := strings.Count(html, "<section>"); got != 4 {
		t.Fatalf("sections = %d, want 4", got)
	}
	if !strings.Contains(html, "<footer>Plan complete</footer>") {
		t.Fatalf("footer missing")
	}
	if !strings.Contains(html, `<h2>Decisions <span>2</span></h2>`) {
		t.Fatalf("heading with count missing")
	}
}

func TestHTMLIncomplete(t *testing.T) {
	html := renderHTML(parse(t, "# T\n\n## Decisions\n- one\n"), tpl(t, "plan"), "x.md", "")
	if !strings.Contains(html, "Missing or empty: Rejected") {
		t.Fatalf("warning missing: %s", html)
	}
}

func TestHTMLEscapesContent(t *testing.T) {
	d := parse(t, "# T\n\n## Decisions\n\n- <script>alert(1)</script> & `x`\n")
	html := renderHTML(d, tpl(t, "plan"), "x.md", "")
	if strings.Contains(html, "<script>alert") {
		t.Fatalf("raw script survived")
	}
	if !strings.Contains(html, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("script not escaped")
	}
	if !strings.Contains(html, "<code>x</code>") {
		t.Fatalf("code span not rendered")
	}
}

func TestSelectionPage(t *testing.T) {
	d := parse(t, "# Pick\n\n## First\n- a\n\n## Second\n- b\n")
	html := renderHTML(d, tpl(t, "selection"), "x.md", "")
	if got := strings.Count(html, `type="radio"`); got != 3 {
		t.Fatalf("radios = %d, want 3", got)
	}
	if !strings.Contains(html, `id="copy"`) {
		t.Fatalf("copy button missing")
	}
	if !strings.Contains(html, `value="A: First"`) {
		t.Fatalf("first option value missing")
	}
}

func TestScanProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "lib"), 0o755)
	os.WriteFile(filepath.Join(dir, "lib", "util.go"), []byte("package lib\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# x\n"), 0o644)

	doc, err := scanProject(dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Sections) != 4 {
		t.Fatalf("sections = %d, want 4", len(doc.Sections))
	}

	tree := strings.Join(doc.find("Tree").Body, "\n")
	if !strings.Contains(tree, "main.go") || !strings.Contains(tree, "lib/") {
		t.Fatalf("tree missing entries:\n%s", tree)
	}

	overview := strings.Join(doc.find("Overview").Body, "\n")
	if !strings.Contains(overview, "3 files") {
		t.Fatalf("overview = %q", overview)
	}

	if got := doc.Missing(tpl(t, "map").Required); len(got) != 0 {
		t.Fatalf("map generated with missing sections: %v", got)
	}
}

func TestNestedBullets(t *testing.T) {
	d := parse(t, "# M\n\n## Modules\n\n- top\n  - child\n")
	html := renderHTML(d, tpl(t, "map"), "x.md", "")
	if !strings.Contains(html, `<li style="--l:1">child</li>`) {
		t.Fatalf("nested level missing: %s", html)
	}
}

func TestInlineMarkup(t *testing.T) {
	got := inline("**bold** and [link](https://example.com) and `code`")
	if !strings.Contains(got, "<strong>bold</strong>") {
		t.Fatalf("bold missing: %s", got)
	}
	if !strings.Contains(got, `<a href="https://example.com">link</a>`) {
		t.Fatalf("link missing: %s", got)
	}
	if !strings.Contains(got, "<code>code</code>") {
		t.Fatalf("code missing: %s", got)
	}
	if strings.Contains(inline("[x](javascript:alert(1))"), "<a ") {
		t.Fatalf("javascript link was rendered")
	}
}

func TestImageInlined(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "shot.png"), []byte("\x89PNG\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d := parse(t, "# T\n\n![a shot](shot.png)\n")
	html := renderHTML(d, tpl(t, "plan"), "x.md", dir)
	if !strings.Contains(html, "data:image/png;base64,") {
		t.Fatalf("image not inlined")
	}
	if !strings.Contains(html, "<figcaption>a shot</figcaption>") {
		t.Fatalf("caption missing")
	}
}

func TestTerminalExitCodes(t *testing.T) {
	var b strings.Builder
	if code := renderTerminal(parse(t, planDoc), tpl(t, "plan"), "x", false, &b); code != 0 {
		t.Fatalf("complete plan exit = %d", code)
	}
	b.Reset()
	if code := renderTerminal(parse(t, "# T\n\n## Decisions\n- one\n"), tpl(t, "plan"), "x", false, &b); code != 1 {
		t.Fatalf("incomplete plan exit = %d", code)
	}
	if !strings.Contains(b.String(), "missing or empty:") {
		t.Fatalf("warning missing from terminal output")
	}
}

func TestDeclaredTemplate(t *testing.T) {
	lines := []string{"<!-- vistan: map -->", "", "# T"}
	if got := DeclaredTemplate(lines); got != "map" {
		t.Fatalf("declared = %q", got)
	}
	if got := DeclaredTemplate([]string{"# T"}); got != "" {
		t.Fatalf("declared = %q, want empty", got)
	}
}

func TestTemplatesAreEmbedded(t *testing.T) {
	for _, tpl := range templateList {
		text, ok := templateText(tpl.Name)
		if !ok {
			t.Fatalf("template %q not embedded", tpl.Name)
		}
		if !strings.Contains(text, "vistan: "+tpl.Name) {
			t.Fatalf("template %q is missing its marker", tpl.Name)
		}
	}
}
