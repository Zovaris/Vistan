package vistan

import (
	"fmt"
	"io"
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
	"Git":           ansiWhite,
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
