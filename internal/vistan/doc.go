package vistan

import (
	"embed"
	"regexp"
	"strings"
)

type Section struct {
	Name string
	Body []string
}

type Doc struct {
	Title    string
	Intro    []string
	Sections []Section
	Artifact string
}

var (
	fenceLine = regexp.MustCompile("^[ \t]*```")
	marker    = regexp.MustCompile(`^[ \t]*<!--[ \t]*vistan:[ \t]*([a-z][a-z-]*)[ \t]*-->[ \t]*$`)
)

func Parse(lines []string) *Doc {
	d := &Doc{}
	cur := -1
	inFence := false

	add := func(line string) {
		if cur >= 0 {
			d.Sections[cur].Body = append(d.Sections[cur].Body, line)
			return
		}
		d.Intro = append(d.Intro, line)
	}

	for _, line := range lines {
		switch {
		case marker.MatchString(line):
			continue
		case fenceLine.MatchString(line):
			inFence = !inFence
			add(line)
		case inFence:
			add(line)
		case strings.HasPrefix(line, "## "):
			d.Sections = append(d.Sections, Section{Name: strings.TrimSpace(line[3:])})
			cur = len(d.Sections) - 1
		case strings.HasPrefix(line, "# "):
			if d.Title == "" {
				d.Title = strings.TrimSpace(line[2:])
			}
		default:
			add(line)
		}
	}
	return d
}

func (s Section) BodyTrimmed() []string { return trimBlank(s.Body) }

func trimBlank(lines []string) []string {
	start, end := 0, len(lines)
	for start < end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[start:end]
}

func (d *Doc) find(name string) *Section {
	for i := range d.Sections {
		if d.Sections[i].Name == name {
			return &d.Sections[i]
		}
	}
	return nil
}

func (d *Doc) Missing(required []string) []string {
	var missing []string
	for _, name := range required {
		s := d.find(name)
		if s == nil || len(s.BodyTrimmed()) == 0 {
			missing = append(missing, name)
		}
	}
	return missing
}

type Template struct {
	Name     string
	Required []string
	Label    string
}

var templateList = []Template{
	{"plan", []string{"Decisions", "Rejected", "Verification", "Your move"}, "Plan complete"},
	{"summary", []string{"What changed", "Verification", "Pending"}, "Summary complete"},
	{"selection", nil, "Decision complete"},
	{"guide", nil, "Guide complete"},
	{"map", []string{"Overview", "Tree", "Entry points", "Largest files"}, "Map complete"},
	{"diagram", []string{"Context", "Diagram", "Reading"}, "Diagram complete"},
	{"graph", []string{"Context", "Graph", "Reading"}, "Graph complete"},
	{"adr", []string{"Status", "Context", "Decision", "Consequences"}, "Decision record complete"},
	{"spec", []string{"Status", "Problem", "Proposal", "Decisions"}, "Spec complete"},
	{"changelog", nil, "Changelog complete"},
	{"review", nil, "Review complete"},
}

func lookupTemplate(name string) (Template, bool) {
	for _, t := range templateList {
		if t.Name == name {
			return t, true
		}
	}
	return Template{}, false
}

//go:embed templates/*.md
var templateFiles embed.FS

//go:embed BRIEF.md
var agentBrief string

func templateText(name string) (string, bool) {
	b, err := templateFiles.ReadFile("templates/" + name + ".md")
	if err != nil {
		return "", false
	}
	return string(b), true
}

func DeclaredTemplate(lines []string) string {
	for _, line := range lines {
		if m := marker.FindStringSubmatch(line); m != nil {
			return m[1]
		}
	}
	return ""
}
