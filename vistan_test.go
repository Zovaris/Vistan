package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	if got := strings.Count(html, "<section"); got != 4 {
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
	if got := inline("**`file://` plus a shim**"); !strings.Contains(got, "<strong><code>file://</code> plus a shim</strong>") {
		t.Fatalf("bold around code = %s", got)
	}
	if got := inline("`a **b** c`"); !strings.Contains(got, "<code>a **b** c</code>") {
		t.Fatalf("bold inside code = %s", got)
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

func TestSelectionOptionWithImageAndCode(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "shot.png"), []byte("\x89PNG\r\n"), 0o644)
	d := parse(t, "# Pick\n\n## A\n\n![shot](shot.png)\n\n```\ncode here\n```\n")
	html := renderHTML(d, tpl(t, "selection"), "x.md", dir)
	if !strings.Contains(html, "data:image/png;base64,") {
		t.Fatalf("option image not inlined")
	}
	if !strings.Contains(html, "<pre><code>\ncode here") {
		t.Fatalf("option code block missing")
	}
}

func TestChangelogReleases(t *testing.T) {
	d := parse(t, "# Changelog\n\n## Unreleased\n\n### Added\n\n- one\n\n## 1.0.0\n\n### Fixed\n\n- two\n")
	html := renderHTML(d, tpl(t, "changelog"), "x.md", "")
	if !strings.Contains(html, "<h3>Added</h3>") || !strings.Contains(html, "<h3>Fixed</h3>") {
		t.Fatalf("changelog subheads missing")
	}
	if !strings.Contains(html, `class="release is-unreleased"`) {
		t.Fatalf("unreleased marker missing")
	}
	if strings.Contains(html, `<span>1</span>`) {
		t.Fatalf("changelog shows bullet counts")
	}
}

func TestAdrStatusChip(t *testing.T) {
	d := parse(t, "# ADR\n\n## Status\n\naccepted\n\n## Context\n\nx\n\n## Decision\n\ny\n\n## Consequences\n\n- z\n")
	html := renderHTML(d, tpl(t, "adr"), "x.md", "")
	if !strings.Contains(html, `class="status is-accepted"`) {
		t.Fatalf("status chip class missing")
	}
}

func TestTreemapArtifact(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "lib"), 0o755)
	os.WriteFile(filepath.Join(dir, "lib", "util.go"), []byte("package lib\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644)

	doc, err := scanProject(dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc.Artifact, "<svg") || !strings.Contains(doc.Artifact, "lib") {
		t.Fatalf("treemap artifact missing: %q", doc.Artifact)
	}
	if html := renderHTML(doc, tpl(t, "map"), "x", ""); !strings.Contains(html, `<div class="art">`) {
		t.Fatalf("treemap not rendered into the page")
	}
}

func TestIndexListsPagesWithLinks(t *testing.T) {
	pages := []pageEntry{
		{name: "plan.html", title: "A plan", tpl: "plan", mod: time.Unix(0, 0)},
		{name: "adr.html", title: "An ADR", tpl: "adr", mod: time.Unix(0, 0)},
	}
	html := indexHTML(pages)
	for _, want := range []string{`href="plan.html"`, `href="adr.html"`, ">plan<", ">adr<"} {
		if !strings.Contains(html, want) {
			t.Fatalf("index missing %q", want)
		}
	}
}

func TestTasksRenderWithProgress(t *testing.T) {
	d := parse(t, "# P\n\n## Steps\n\n- [x] one\n- [x] two\n- [ ] three\n")
	html := renderHTML(d, tpl(t, "plan"), "x.md", "")
	if !strings.Contains(html, `<h2>Steps <span>2/3</span></h2>`) {
		t.Fatalf("task progress missing: %s", html)
	}
	if got := strings.Count(html, `class="task done"`); got != 2 {
		t.Fatalf("done tasks = %d, want 2", got)
	}
	if !strings.Contains(html, `<span class="box">&#9744;</span>`) {
		t.Fatalf("open task box missing")
	}
}

func TestCallout(t *testing.T) {
	d := parse(t, "# P\n\n## Context\n\n> [!WARN] Careful now\n> It is one line\n> continued here.\n>\n> - and a bullet\n")
	html := renderHTML(d, tpl(t, "plan"), "x.md", "")
	if !strings.Contains(html, `<aside class="callout warn">`) {
		t.Fatalf("callout missing: %s", html)
	}
	if !strings.Contains(html, `<p class="kind">Warning</p>`) {
		t.Fatalf("callout label missing")
	}
	if !strings.Contains(html, "<p>It is one line continued here.</p>") {
		t.Fatalf("callout paragraph not joined")
	}
	if !strings.Contains(html, "<li>and a bullet</li>") {
		t.Fatalf("callout list missing")
	}
}

func TestChangesBlock(t *testing.T) {
	d := parse(t, "# P\n\n## Files\n\n```changes\n+ a.go\n~ b.go  why it changed\n- c.go\n> old.go -> new.go\n```\n")
	html := renderHTML(d, tpl(t, "plan"), "x.md", "")
	for _, want := range []string{
		`<ul class="changes">`,
		`class="add"><span class="mark">A</span><code>a.go</code>`,
		`class="mod"><span class="mark">M</span><code>b.go</code>`,
		`class="note">— why it changed</span>`,
		`class="del"><span class="mark">D</span><code>c.go</code>`,
		`class="move"><span class="mark">R</span><code>old.go -&gt; new.go</code>`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("changes block missing %q", want)
		}
	}
}

func TestStatsStrip(t *testing.T) {
	d := parse(t, "# P\n\n## Steps\n\n- [x] one\n- [ ] two\n\n## Files\n\n```changes\n+ a.go\n```\n\n## Risk\n\n> [!RISK] no auth\n> loopback only\n\n## Your move\n\n- decide\n")
	html := renderHTML(d, tpl(t, "plan"), "x.md", "")
	for _, want := range []string{"<b>1/2</b> checks", "<b>1</b> file", "<b>1</b> risk", `class="wait"`} {
		if !strings.Contains(html, want) {
			t.Fatalf("stats missing %q in %s", want, html)
		}
	}
}

func TestYourMoveIsAPanel(t *testing.T) {
	d := parse(t, planDoc)
	html := renderHTML(d, tpl(t, "plan"), "x.md", "")
	if !strings.Contains(html, `<section class="move">`) {
		t.Fatalf("move panel missing")
	}
}

func TestTerminalTasksAndCallouts(t *testing.T) {
	d := parse(t, "# P\n\n## Steps\n\n- [x] one\n- [ ] two\n\n## Risk\n\n> [!WARN] no auth\n> loopback only\n")
	var b strings.Builder
	renderTerminal(d, tpl(t, "plan"), "x", false, &b)
	out := b.String()
	for _, want := range []string{"[x] one", "[ ] two", "! warning — no auth", "loopback only"} {
		if !strings.Contains(out, want) {
			t.Fatalf("terminal missing %q in %s", want, out)
		}
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
