package vistan

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
	if !strings.Contains(html, `id="send"`) || !strings.Contains(html, "sendAnswer") {
		t.Fatalf("send button missing")
	}
	if !strings.Contains(html, "section.opt") {
		t.Fatalf("the whole option card is not clickable")
	}
	if !strings.Contains(html, `value="A: First"`) {
		t.Fatalf("first option value missing")
	}
}

func TestAnswerEndpointStoresTheChoice(t *testing.T) {
	dir := t.TempDir()
	req := httptest.NewRequest("POST", "/_answer/pick", strings.NewReader(`{"answer":"B: Postgres"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	answerHandler(dir).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	b, err := os.ReadFile(filepath.Join(dir, "pick.answer.json"))
	if err != nil {
		t.Fatal(err)
	}
	var stored answer
	if err := json.Unmarshal(b, &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Answer != "B: Postgres" || stored.Page != "pick" || stored.At.IsZero() {
		t.Fatalf("stored = %+v", stored)
	}
}

func TestAnswerEndpointRefusesAForeignOrigin(t *testing.T) {
	dir := t.TempDir()
	req := httptest.NewRequest("POST", "/_answer/pick", strings.NewReader(`{"answer":"x"}`))
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	answerHandler(dir).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(dir, "pick.answer.json")); !os.IsNotExist(err) {
		t.Fatalf("a foreign origin stored an answer")
	}
}

func TestAnswerCommandPrintsTheChoice(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VISTAN_CACHE", dir)
	page := "<!doctype html><html><head><title>Pick</title></head><body class=\"tpl-selection\"><h1>Pick</h1></body></html>"
	if err := os.WriteFile(filepath.Join(dir, "pick.html"), []byte(page), 0o644); err != nil {
		t.Fatal(err)
	}
	stored := `{"page":"pick","answer":"B: Postgres","at":"2026-10-22T10:00:00Z"}`
	if err := os.WriteFile(filepath.Join(dir, "pick.answer.json"), []byte(stored), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if code := Run([]string{"answer", "pick"}); code != 0 {
			t.Fatalf("answer = %d, want 0", code)
		}
	})
	if strings.TrimSpace(out) != "B: Postgres" {
		t.Fatalf("answer printed %q", out)
	}
}

func TestAnswerCommandWithoutAnAnswer(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VISTAN_CACHE", dir)
	page := "<!doctype html><html><head><title>Pick</title></head><body class=\"tpl-selection\"><h1>Pick</h1></body></html>"
	if err := os.WriteFile(filepath.Join(dir, "pick.html"), []byte(page), 0o644); err != nil {
		t.Fatal(err)
	}

	if code := Run([]string{"answer", "pick"}); code != 1 {
		t.Fatalf("answer with no answer = %d, want 1", code)
	}
}

func TestScanProjectGitSection(t *testing.T) {
	if _, err := gitOutput(".", "--version"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	if _, err := gitOutput(dir, "init", "-q"); err != nil {
		t.Skipf("git init: %v", err)
	}
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644)
	gitOutput(dir, "add", "main.go")
	if _, err := gitOutput(dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "first page", "--no-gpg-sign"); err != nil {
		t.Skipf("git commit: %v", err)
	}
	os.WriteFile(filepath.Join(dir, "notes.md"), []byte("# note\n"), 0o644)

	doc, err := scanProject(dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	section := doc.find("Git")
	if section == nil {
		t.Fatal("no Git section")
	}
	body := strings.Join(section.Body, "\n")
	for _, want := range []string{"- `branch` `", "- `commit` `", "first page", "- 1 file changed", "notes.md"} {
		if !strings.Contains(body, want) {
			t.Fatalf("git section missing %q:\n%s", want, body)
		}
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

func TestEntryLinesReadTheProjectName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"vistan\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, _ := collectFiles(dir)
	body := strings.Join(entryLines(dir, files), "\n")
	if !strings.Contains(body, "`Cargo.toml`: vistan") {
		t.Fatalf("project name lost its letters:\n%s", body)
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
	for _, want := range []string{"<svg class=\"tm-map\"", "lib", "<title>", `class="tm-cap"`, `class="tm-rows"`, "<script>"} {
		if !strings.Contains(doc.Artifact, want) {
			t.Fatalf("treemap artifact missing %q: %q", want, doc.Artifact)
		}
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

func TestReviewFindings(t *testing.T) {
	d := parse(t, "# R\n\n## Tests\n\n- [x] go test\n- [ ] e2e\n\n## F1 · Blocker · `server.go:42`\n\nBroken.\n\n**Fix:** compare safely.\n\n## Notes\n\n- nothing else\n")
	html := renderHTML(d, tpl(t, "review"), "x.md", "")
	for _, want := range []string{
		`class="finding sev-blocker" id="f1"`,
		`<span class="sev">Blocker</span>`,
		`<code class="loc">server.go:42</code>`,
		`value="F1"`,
		`<h2>Tests <span>1/2</span></h2>`,
		`<h2>Notes <span>1</span></h2>`,
		`<strong>Fix:</strong> compare safely.`,
		`id="ans"`,
		`section.finding`,
		`<b>1</b> finding`,
		`<b>1</b> blocker`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("review page missing %q", want)
		}
	}
}

func TestReviewWithoutFindingsHasNoDecisionPanel(t *testing.T) {
	d := parse(t, "# R\n\n## Tests\n\n- [x] go test\n")
	html := renderHTML(d, tpl(t, "review"), "x.md", "")
	if strings.Contains(html, `id="ans"`) {
		t.Fatalf("decision panel rendered with no findings")
	}
}

func TestParseFinding(t *testing.T) {
	id, severity, location, ok := parseFinding("F2 · Major · `render.go:88`")
	if !ok || id != "F2" || severity != "Major" || location != "render.go:88" {
		t.Fatalf("parseFinding = %q %q %q %v", id, severity, location, ok)
	}
	if _, _, _, ok := parseFinding("Notes"); ok {
		t.Fatalf("plain section parsed as a finding")
	}
	if sev := sevSlug("Pre-existing"); sev != "pre-existing" {
		t.Fatalf("sevSlug = %q", sev)
	}
}

func TestBriefIsEmbedded(t *testing.T) {
	for _, want := range []string{"vistan new review", "## Commands", "Pick the template"} {
		if !strings.Contains(agentBrief, want) {
			t.Fatalf("brief missing %q", want)
		}
	}
}

func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestHelpWorksAfterAnyCommand(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"help"}, {"serve", "--help"}, {"rerender", "--all", "-h"}} {
		var code int
		out := captureStdout(t, func() { code = Run(args) })
		if code != 0 {
			t.Fatalf("Run(%v) = %d, want 0", args, code)
		}
		if !strings.HasPrefix(out, "usage: vistan") {
			t.Fatalf("Run(%v) printed %q, want the usage", args, out)
		}
	}
}

func TestVersionIsPrinted(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--version"}} {
		var code int
		out := captureStdout(t, func() { code = Run(args) })
		if code != 0 || out != "vistan "+versionString()+"\n" {
			t.Fatalf("Run(%v) = %d, %q; want 0 and the version", args, code, out)
		}
	}
}

func TestVersionFromBuildInfo(t *testing.T) {
	cases := map[string]string{
		"v0.1.0":                            "0.1.0",
		"v1.2.3":                            "1.2.3",
		"v0.1.0-rc1":                        "",
		"0.0.0-20261023054514-9ee7305a5429": "",
		"0.0.0-20261023054514-9ee7305a5429+dirty": "",
		"(devel)": "",
	}
	for in, want := range cases {
		if got := versionFromBuildInfo(in); got != want {
			t.Fatalf("versionFromBuildInfo(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWithReloadInjectsTheScript(t *testing.T) {
	page := "<html><body><p>x</p></body></html>"
	got := withReload(page)
	if !strings.Contains(got, "setInterval") || !strings.HasSuffix(got, "</body></html>") {
		t.Fatalf("the reload script was not put before </body>: %s", got)
	}
	if tail := withReload("<p>x</p>"); !strings.HasSuffix(tail, reloadScript) {
		t.Fatalf("a page without a body did not get the script: %s", tail)
	}
}

func TestWatchNeedsServeAndAFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "page.md")
	if err := os.WriteFile(path, []byte("# T\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := Run([]string{"--watch", path}); code != 2 {
		t.Fatalf("--watch without serve = %d, want 2", code)
	}
	if code := Run([]string{"serve", "--watch", "-"}); code != 2 {
		t.Fatalf("--watch with stdin = %d, want 2", code)
	}
}

func TestSplitFlags(t *testing.T) {
	got := strings.Join(splitFlags([]string{"--depth=3", "-x=1", "page.md", "--template=a=b", "--"}), "|")
	want := "--depth|3|-x=1|page.md|--template|a=b|--"
	if got != want {
		t.Fatalf("splitFlags = %q, want %q", got, want)
	}
}

func TestFlagWithEqualsReachesTheCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "page.md")
	if err := os.WriteFile(path, []byte("# T\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var code int
	captureStdout(t, func() { code = Run([]string{"--template=nope", path}) })
	if code != 2 {
		t.Fatalf("--template=nope = %d, want 2 from the unknown template", code)
	}
}

func TestBriefEndsWithTheUsage(t *testing.T) {
	out := captureStdout(t, func() { Run([]string{"brief"}) })
	if !strings.HasPrefix(out, "# vistan brief") || !strings.Contains(out, "\nusage: vistan") {
		t.Fatalf("brief did not print the brief followed by the usage")
	}
}

func TestCleanRemovesTheState(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "plan.html"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISTAN_CACHE", state)

	out := captureStdout(t, func() {
		if code := Run([]string{"clean"}); code != 0 {
			t.Fatalf("clean = %d, want 0", code)
		}
	})
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatalf("state directory survived clean")
	}
	if !strings.Contains(out, "removed") {
		t.Fatalf("clean printed %q", out)
	}

	out = captureStdout(t, func() {
		if code := Run([]string{"clean"}); code != 0 {
			t.Fatalf("clean on an empty state = %d, want 0", code)
		}
	})
	if !strings.Contains(out, "nothing to remove") {
		t.Fatalf("second clean printed %q", out)
	}
}

func TestRemoveStateRefusesTheWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	if err := removeState("."); err == nil {
		t.Fatalf("removeState deleted the working directory")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("working directory is gone: %v", err)
	}
}

func TestStateDirHonoursCacheEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VISTAN_CACHE", dir)
	if got := stateDir(); got != dir {
		t.Fatalf("stateDir = %q, want %q", got, dir)
	}
}

func TestKeepPagesPrunesOldest(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("page-%d.html", i)
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("page-%d.md", i)), []byte("# x\n"), 0o644)
		stamp := now.Add(time.Duration(i-4) * time.Hour)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("index"), 0o644)

	t.Setenv("VISTAN_KEEP", "3")
	keepPages(dir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 7 {
		t.Fatalf("files left = %d, want 7 (3 pages, 3 sources, index)", len(entries))
	}
	for _, name := range []string{"page-0.html", "page-1.html", "page-0.md", "page-1.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s should have been pruned", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err != nil {
		t.Fatalf("index.html was pruned")
	}
	if _, err := os.Stat(filepath.Join(dir, "page-4.html")); err != nil {
		t.Fatalf("newest page was pruned")
	}
	if _, err := os.Stat(filepath.Join(dir, "page-4.md")); err != nil {
		t.Fatalf("source of the newest page was pruned")
	}
}

func TestKeepPagesPrunesPerProject(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	write := func(name, project string, age time.Duration) {
		html := fmt.Sprintf(`<head><meta name="vistan-project" content=%q></head>`, project)
		path := filepath.Join(dir, name+".html")
		if err := os.WriteFile(path, []byte(html), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte("# x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		stamp := now.Add(-age)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	write("alpha-0", "alpha", 4*time.Hour)
	write("alpha-1", "alpha", 3*time.Hour)
	write("alpha-2", "alpha", 2*time.Hour)
	write("beta-0", "beta", 5*time.Hour)
	write("beta-1", "beta", 4*time.Hour)
	write("beta-2", "beta", time.Hour)

	t.Setenv("VISTAN_KEEP", "2")
	t.Setenv("VISTAN_KEEP_DAYS", "0")
	keepPages(dir)

	for _, gone := range []string{"alpha-0", "beta-0"} {
		if _, err := os.Stat(filepath.Join(dir, gone+".html")); !os.IsNotExist(err) {
			t.Fatalf("%s should have been pruned", gone)
		}
	}
	for _, kept := range []string{"alpha-1", "alpha-2", "beta-1", "beta-2"} {
		if _, err := os.Stat(filepath.Join(dir, kept+".html")); err != nil {
			t.Fatalf("%s should have been kept: %v", kept, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "beta-2.md")); err != nil {
		t.Fatalf("source of a kept page was pruned")
	}
}

func TestKeepPagesUnlimitedWhenZero(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 3; i++ {
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("p%d.html", i)), []byte("x"), 0o644)
	}
	t.Setenv("VISTAN_KEEP", "0")
	t.Setenv("VISTAN_KEEP_DAYS", "0")
	keepPages(dir)
	entries, _ := os.ReadDir(dir)
	if len(entries) != 3 {
		t.Fatalf("files left = %d, want 3", len(entries))
	}
}

func TestKeepPagesPrunesByAge(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.html")
	fresh := filepath.Join(dir, "fresh.html")
	os.WriteFile(old, []byte("x"), 0o644)
	os.WriteFile(fresh, []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "old.md"), []byte("# x\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "fresh.md"), []byte("# x\n"), 0o644)
	stamp := time.Now().AddDate(0, 0, -40)
	if err := os.Chtimes(old, stamp, stamp); err != nil {
		t.Fatal(err)
	}

	t.Setenv("VISTAN_KEEP", "0")
	keepPages(dir)

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("page older than 30 days was kept")
	}
	if _, err := os.Stat(filepath.Join(dir, "old.md")); !os.IsNotExist(err) {
		t.Fatalf("source of the old page was kept")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("recent page was pruned: %v", err)
	}
}

func TestLastSeen(t *testing.T) {
	now := time.Now()
	if got := lastSeen(now); !strings.HasPrefix(got, "today ") {
		t.Fatalf("today label = %q", got)
	}
	if got := lastSeen(now.AddDate(0, 0, -1)); got == "" || strings.HasPrefix(got, "today ") {
		t.Fatalf("yesterday label = %q", got)
	}
	if got := lastSeen(now.AddDate(-1, 0, 0)); !strings.Contains(got, " ") {
		t.Fatalf("old label = %q", got)
	}
}

func TestActivityStrip(t *testing.T) {
	now := time.Now()
	pages := []pageEntry{
		{mod: now}, {mod: now}, {mod: now},
		{mod: now.AddDate(0, 0, -3)},
	}
	strip := activityStrip(pages)
	if got := strings.Count(strip, `class="cell`); got != 30 {
		t.Fatalf("cells = %d, want 30", got)
	}
	if !strings.Contains(strip, fmt.Sprintf(`class="cell l3" title="%s · 3 pages"`, now.Format("2 Jan"))) {
		t.Fatalf("today is not the darkest cell: %s", strip)
	}
	if !strings.Contains(strip, `title="`+now.AddDate(0, 0, -3).Format("2 Jan")+` · 1 page"`) {
		t.Fatalf("single-page day missing or mislabelled")
	}
	if empty := activityStrip(nil); strings.Count(empty, `l0`) != 30 {
		t.Fatalf("empty history is not thirty empty cells")
	}
}

func TestIndexGroupsByProject(t *testing.T) {
	now := time.Now()
	pages := []pageEntry{
		{name: "plan.html", title: "A plan", tpl: "plan", project: "vistan", mod: now},
		{name: "review.html", title: "A review", tpl: "review", project: "vistan", mod: now.Add(-time.Hour)},
		{name: "spec.html", title: "A spec", tpl: "spec", project: "fizza", mod: now.AddDate(0, 0, -1)},
	}
	html := indexHTML(pages)
	for _, want := range []string{
		`<h2 class="proj">vistan`, "2 pages · last today", `<h2 class="proj">fizza`, "1 page · last",
		"3 pages · 2 projects", `href="review.html"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("index missing %q", want)
		}
	}
	if strings.Index(html, ">vistan ") > strings.Index(html, ">fizza ") {
		t.Fatalf("projects are not ordered by last activity")
	}
	if strings.Contains(html, "Today<") {
		t.Fatalf("index still groups by day")
	}
}

func TestWritePageKeepsTheSource(t *testing.T) {
	dir := t.TempDir()
	if err := writePage(dir, "plan", "<html></html>", []string{"# A", "", "text"}); err != nil {
		t.Fatal(err)
	}
	md, err := os.ReadFile(filepath.Join(dir, "plan.md"))
	if err != nil {
		t.Fatalf("source not written: %v", err)
	}
	if string(md) != "# A\n\ntext\n" {
		t.Fatalf("source = %q", md)
	}
	if err := writePage(dir, "map", "<html></html>", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "map.md")); !os.IsNotExist(err) {
		t.Fatalf("generated page wrote a source it does not have")
	}
}

func TestRerenderKeepsTheLabelAndTemplate(t *testing.T) {
	dir := t.TempDir()
	lines := strings.Split("<!-- vistan: graph -->\n\n# G\n\n## Context\n\nx\n\n## Graph\n\n```\na --> b\n```\n\n## Reading\n\n- y", "\n")
	first := renderHTML(Parse(lines), tpl(t, "graph"), "examples/graph.md", "examples")
	if err := writePage(dir, "graph", first, lines); err != nil {
		t.Fatal(err)
	}

	page, got, err := rerenderHTML(dir, "graph")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page, `<svg class="graph"`) {
		t.Fatalf("template from the marker was not used")
	}
	if !strings.Contains(page, `content="examples/graph.md"`) {
		t.Fatalf("original source label was not preserved")
	}
	if len(got) != len(lines) {
		t.Fatalf("lines = %d, want %d", len(got), len(lines))
	}
}

func TestRerenderWithoutSource(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "map.html"), []byte(`<html><body class="tpl-map"><h1>M</h1></body></html>`), 0o644)
	if _, _, err := rerenderHTML(dir, "map"); err == nil {
		t.Fatalf("rerendered a page with no stored source")
	}
}

func TestRerenderAllSkipsPagesWithoutSource(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VISTAN_CACHE", dir)
	lines := strings.Split("<!-- vistan: plan -->\n\n# P\n\n## Decisions\n\n- a", "\n")
	if err := writePage(dir, "plan", `<html><body class="tpl-plan"><h1>stale</h1></body></html>`, lines); err != nil {
		t.Fatal(err)
	}
	mapPage := `<html><body class="tpl-map"><h1>M</h1></body></html>`
	if err := os.WriteFile(filepath.Join(dir, "map.html"), []byte(mapPage), 0o644); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	os.Chtimes(filepath.Join(dir, "plan.html"), stamp, stamp)

	if code := rerenderAll(); code != 0 {
		t.Fatalf("rerender --all = %d", code)
	}

	b, err := os.ReadFile(filepath.Join(dir, "plan.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "stale") || !strings.Contains(string(b), "<h1>P</h1>") {
		t.Fatalf("page was not rebuilt from its source")
	}
	if info, err := os.Stat(filepath.Join(dir, "plan.html")); err != nil || !info.ModTime().Equal(stamp) {
		t.Fatalf("rerender --all reset the page date: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "map.html")); err != nil || string(b) != mapPage {
		t.Fatalf("page without source was touched: %v", err)
	}
}

func TestHistoryJSON(t *testing.T) {
	dir := t.TempDir()
	page := `<html><body class="tpl-plan"><h1>A &amp; B</h1><meta name="vistan-project" content="demo"></body></html>`
	if err := os.WriteFile(filepath.Join(dir, "plan.html"), []byte(page), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plan.md"), []byte("# A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pages, err := collectPages(dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := historyJSON(dir, pages)
	if err != nil {
		t.Fatal(err)
	}

	var got struct {
		Dir   string `json:"dir"`
		Pages []struct {
			Name     string `json:"name"`
			Title    string `json:"title"`
			Template string `json:"template"`
			Project  string `json:"project"`
			Source   string `json:"source"`
			Modified string `json:"modified"`
			Bytes    int64  `json:"bytes"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, b)
	}
	if got.Dir != dir || len(got.Pages) != 1 {
		t.Fatalf("history = %+v", got)
	}
	entry := got.Pages[0]
	if entry.Name != "plan.html" || entry.Title != "A & B" || entry.Template != "plan" || entry.Project != "demo" {
		t.Fatalf("entry = %+v", entry)
	}
	if entry.Source != "plan.md" {
		t.Fatalf("source = %q", entry.Source)
	}
	if entry.Bytes != int64(len(page)) {
		t.Fatalf("bytes = %d, want %d", entry.Bytes, len(page))
	}
	if _, err := time.Parse(time.RFC3339, entry.Modified); err != nil {
		t.Fatalf("modified = %q", entry.Modified)
	}

	empty, err := historyJSON(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(empty), `"pages": []`) {
		t.Fatalf("empty history is not an empty array: %s", empty)
	}
}

func TestResolvePage(t *testing.T) {
	dir := t.TempDir()
	body := []byte(`<html><body class="tpl-review"><h1>A page</h1></body></html>`)
	for _, name := range []string{"plan.html", "review-treemap.html", "review-history.html"} {
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if path, _ := resolvePage(dir, "plan"); path != filepath.Join(dir, "plan.html") {
		t.Fatalf("exact match = %q", path)
	}
	if path, _ := resolvePage(dir, "plan.html"); path != filepath.Join(dir, "plan.html") {
		t.Fatalf("extension match = %q", path)
	}
	if path, _ := resolvePage(dir, "treemap"); path != filepath.Join(dir, "review-treemap.html") {
		t.Fatalf("contains match = %q", path)
	}
	if path, candidates := resolvePage(dir, "review"); path != "" || len(candidates) != 2 {
		t.Fatalf("ambiguous = %q %v", path, candidates)
	}
	if path, _ := resolvePage(dir, "nope"); path != "" {
		t.Fatalf("missing match = %q", path)
	}
}

func TestRemovePage(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VISTAN_CACHE", dir)
	for _, name := range []string{"plan.html", "keep.html"} {
		os.WriteFile(filepath.Join(dir, name), []byte(`<html><body class="tpl-plan"><h1>x</h1></body></html>`), 0o644)
		os.WriteFile(filepath.Join(dir, strings.TrimSuffix(name, ".html")+".md"), []byte("# x\n"), 0o644)
	}
	if code := runRm([]string{"plan"}); code != 0 {
		t.Fatalf("runRm exit = %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "plan.html")); !os.IsNotExist(err) {
		t.Fatalf("plan.html still exists")
	}
	if _, err := os.Stat(filepath.Join(dir, "plan.md")); !os.IsNotExist(err) {
		t.Fatalf("plan.md still exists")
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.html")); err != nil {
		t.Fatalf("keep.html was removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.md")); err != nil {
		t.Fatalf("keep.md was removed")
	}
	if code := runRm([]string{"missing"}); code != 2 {
		t.Fatalf("missing page exit = %d", code)
	}
}

func TestGraphSVG(t *testing.T) {
	svg := graphSVG([]string{"web --> api : http", "api --> db", "db --> api", "api [API server]"})
	for _, want := range []string{`<svg class="graph"`, ">API server<", ">web<", ">http<", `marker-end="url(#g-arrow)"`} {
		if !strings.Contains(svg, want) {
			t.Fatalf("graph missing %q in %s", want, svg)
		}
	}
	nodes, edges := parseGraph([]string{"a -> b", "b -> a", "a [Alpha]"})
	if len(nodes) != 2 || len(edges) != 2 || nodes[0].label != "Alpha" {
		t.Fatalf("parseGraph = %v %v", nodes, edges)
	}
}

func TestGraphTemplateReplacesTheEdgeList(t *testing.T) {
	d := parse(t, "# G\n\n## Context\n\nx\n\n## Graph\n\n```\na --> b\n```\n\n## Reading\n\n- y\n")
	html := renderHTML(d, tpl(t, "graph"), "x.md", "")
	if !strings.Contains(html, `<svg class="graph"`) {
		t.Fatalf("graph section did not render as SVG")
	}
	if strings.Contains(html, "<pre>") {
		t.Fatalf("graph section still renders the edge list as code")
	}
}

func TestDocsIndex(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "docs"), 0o755)
	os.WriteFile(filepath.Join(dir, "docs", "serve.md"), []byte("<!-- vistan: adr -->\n\n# Serve over HTTP\n\n## Status\n\naccepted — 20 Oct 2026\n\n## Context\n\nx\n\n## Decision\n\ny\n\n## Consequences\n\n- z\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "docs", "history.md"), []byte("<!-- vistan: spec -->\n\n# Keep pages\n\n## Status\n\ndraft — 22 Oct 2026\n\n## Problem\n\nx\n\n## Proposal\n\ny\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "docs", "notes.md"), []byte("# Notes\n\n- something\n"), 0o644)

	entries, err := collectDocs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	if entries[0].status != "accepted" || entries[0].when != "20 Oct 2026" {
		t.Fatalf("first entry = %+v", entries[0])
	}

	page := decisionsHTML(entries)
	for _, want := range []string{"2 records", "accepted", "draft", `href="docs-serve.html"`, "20 Oct 2026", ">SPEC<", ">ADR<"} {
		if !strings.Contains(page, want) {
			t.Fatalf("decisions page missing %q", want)
		}
	}
	if strings.Contains(page, "notes") {
		t.Fatalf("plain markdown leaked into the index")
	}
}

func TestPagesCarryBackLinkAndFavicon(t *testing.T) {
	d := parse(t, planDoc)
	page := renderHTML(d, tpl(t, "plan"), "x.md", "")
	for _, want := range []string{`class="back" href="/"`, `rel="icon"`} {
		if !strings.Contains(page, want) {
			t.Fatalf("page missing %q", want)
		}
	}

	list := indexHTML([]pageEntry{{name: "plan.html", title: "A plan", tpl: "plan"}})
	if !strings.Contains(list, `rel="icon"`) {
		t.Fatalf("history page missing the favicon")
	}
	if strings.Contains(list, `class="back"`) {
		t.Fatalf("history page should not link back to itself")
	}
}

func TestRefreshIndexListsNewPages(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "plan.html"), []byte(`<html><body class="tpl-plan"><h1>A plan</h1></body></html>`), 0o644)
	refreshIndex(dir)
	b, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `href="plan.html"`) {
		t.Fatalf("index does not list the new page")
	}
}

func TestPageCarriesProjectMeta(t *testing.T) {
	d := parse(t, planDoc)
	html := renderHTML(d, tpl(t, "plan"), "x.md", "")
	if !strings.Contains(html, `<meta name="vistan-project" content="`) {
		t.Fatalf("project meta missing")
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

func TestOrderedList(t *testing.T) {
	html := renderHTML(parse(t, "# T\n\n## Steps\n\n1. first\n2. second\n\n- after\n"), tpl(t, "guide"), "x", "")
	for _, want := range []string{"<ol>", "</ol>", `<li style="--l:0">first</li>`, "<ul>", "after"} {
		if !strings.Contains(html, want) {
			t.Fatalf("ordered list missing %q:\n%s", want, html)
		}
	}
}

func TestAssetName(t *testing.T) {
	if got := assetName("v0.1.1", "darwin", "arm64"); got != "vistan_0.1.1_darwin_arm64.tar.gz" {
		t.Fatalf("unexpected archive name %q", got)
	}
	if got := assetName("v0.1.1", "windows", "amd64"); got != "" {
		t.Fatalf("windows has no archive, got %q", got)
	}
}

func TestChecksumFor(t *testing.T) {
	sums := "aa11  vistan_0.1.1_darwin_arm64.tar.gz\nbb22  checksums.txt\n"
	if got := checksumFor(sums, "vistan_0.1.1_darwin_arm64.tar.gz"); got != "aa11" {
		t.Fatalf("expected aa11, got %q", got)
	}
	if got := checksumFor(sums, "vistan_0.1.1_linux_amd64.tar.gz"); got != "" {
		t.Fatalf("expected no checksum, got %q", got)
	}
}

func TestBinaryFromTarGz(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, name := range []string{"README.md", "vistan"} {
		body := []byte("body of " + name)
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gz.Close()

	got, err := binaryFromTarGz(buf.Bytes(), "vistan")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "body of vistan" {
		t.Fatalf("unexpected contents %q", got)
	}
	if _, err := binaryFromTarGz(buf.Bytes(), "missing"); err == nil {
		t.Fatal("expected an error for a missing entry")
	}
}
