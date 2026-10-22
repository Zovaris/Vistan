package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const maxCountSize = 2 << 20

var skipDirs = map[string]bool{
	"node_modules": true, ".git": true, "vendor": true, "dist": true, "build": true,
	"target": true, ".next": true, "out": true, "__pycache__": true, ".venv": true,
	"venv": true, ".idea": true, ".cache": true, "coverage": true,
}

var textExt = map[string]bool{
	"go": true, "ts": true, "tsx": true, "js": true, "jsx": true, "mjs": true,
	"md": true, "css": true, "scss": true, "html": true, "json": true, "yml": true,
	"yaml": true, "sh": true, "rs": true, "py": true, "c": true, "h": true,
	"cpp": true, "hpp": true, "java": true, "rb": true, "swift": true, "kt": true,
	"toml": true, "sql": true, "txt": true, "svg": true,
}

type scannedFile struct {
	rel   string
	size  int64
	lines int
}

type extStat struct {
	files int
	lines int
	bytes int64
}

func scanProject(root string, depth int) (*Doc, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", abs)
	}

	files, source := collectFiles(abs)
	stats := map[string]*extStat{}
	for _, f := range files {
		ext := strings.TrimPrefix(filepath.Ext(f.rel), ".")
		if ext == "" {
			ext = "(none)"
		}
		s := stats[ext]
		if s == nil {
			s = &extStat{}
			stats[ext] = s
		}
		s.files++
		s.lines += f.lines
		s.bytes += f.size
	}

	doc := &Doc{Title: filepath.Base(abs)}
	doc.Sections = []Section{
		{Name: "Overview", Body: overviewLines(files, stats, source)},
		{Name: "Tree", Body: treeLines(filepath.Base(abs), files, depth)},
		{Name: "Entry points", Body: entryLines(abs, files)},
		{Name: "Largest files", Body: largestLines(files)},
	}
	return doc, nil
}

func collectFiles(root string) ([]scannedFile, string) {
	out, err := exec.Command("git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard", "-z").Output()
	if err == nil && len(out) > 0 {
		var files []scannedFile
		for _, rel := range strings.Split(string(out), "\x00") {
			if rel == "" {
				continue
			}
			if f, ok := readFileInfo(root, rel); ok {
				files = append(files, f)
			}
		}
		if len(files) > 0 {
			return files, "git ls-files"
		}
	}

	var files []scannedFile
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		if f, ok := readFileInfo(root, rel); ok {
			files = append(files, f)
		}
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })
	return files, "walk"
}

func readFileInfo(root, rel string) (scannedFile, bool) {
	info, err := os.Stat(filepath.Join(root, rel))
	if err != nil || info.IsDir() {
		return scannedFile{}, false
	}
	f := scannedFile{rel: rel, size: info.Size()}
	ext := strings.TrimPrefix(filepath.Ext(rel), ".")
	if textExt[ext] && info.Size() <= maxCountSize {
		if b, err := os.ReadFile(filepath.Join(root, rel)); err == nil {
			f.lines = bytes.Count(b, []byte("\n"))
		}
	}
	return f, true
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f kB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func overviewLines(files []scannedFile, stats map[string]*extStat, source string) []string {
	var totalBytes int64
	var totalLines int
	for _, f := range files {
		totalBytes += f.size
		totalLines += f.lines
	}

	lines := []string{
		fmt.Sprintf("- %d files, %s, %d lines counted", len(files), humanSize(totalBytes), totalLines),
		fmt.Sprintf("- scanned with %s", source),
	}

	type pair struct {
		ext string
		s   *extStat
	}
	var ranked []pair
	for ext, s := range stats {
		if s.lines > 0 {
			ranked = append(ranked, pair{ext, s})
		}
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].s.lines > ranked[j].s.lines })
	if len(ranked) > 0 {
		lines = append(lines, "", "```")
		for i, p := range ranked {
			if i == 5 {
				break
			}
			pct := 100 * p.s.lines / max(totalLines, 1)
			label := fmt.Sprintf("%d%%", pct)
			if pct == 0 {
				label = "<1%"
			}
			width := max(pct*24/100, 1)
			lines = append(lines, fmt.Sprintf("%-5s %-24s %4s", p.ext, strings.Repeat("█", width), label))
		}
		lines = append(lines, "```")
	}
	return lines
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

type treeNode struct {
	name  string
	dirs  map[string]*treeNode
	files []string
	count int
	size  int64
}

func buildTree(files []scannedFile) *treeNode {
	root := &treeNode{dirs: map[string]*treeNode{}}
	for _, f := range files {
		parts := strings.Split(filepath.ToSlash(f.rel), "/")
		node := root
		node.count++
		node.size += f.size
		for _, dir := range parts[:len(parts)-1] {
			child := node.dirs[dir]
			if child == nil {
				child = &treeNode{name: dir, dirs: map[string]*treeNode{}}
				node.dirs[dir] = child
			}
			child.count++
			child.size += f.size
			node = child
		}
		node.files = append(node.files, parts[len(parts)-1])
	}
	return root
}

func treeLines(base string, files []scannedFile, maxDepth int) []string {
	root := buildTree(files)
	out := []string{"```", base + "/"}
	walkTree(root, "", 0, maxDepth, &out)
	out = append(out, "```")
	return out
}

func walkTree(n *treeNode, prefix string, depth, maxDepth int, out *[]string) {
	var dirNames []string
	for name := range n.dirs {
		dirNames = append(dirNames, name)
	}
	sort.Strings(dirNames)
	sort.Strings(n.files)

	type entry struct {
		name  string
		isDir bool
	}
	var entries []entry
	for _, d := range dirNames {
		entries = append(entries, entry{d, true})
	}
	for _, f := range n.files {
		entries = append(entries, entry{f, false})
	}

	overflow := 0
	if len(entries) > 14 {
		overflow = len(entries) - 13
		entries = entries[:13]
	}

	for i, e := range entries {
		last := i == len(entries)-1 && overflow == 0
		branch, next := "├── ", "│   "
		if last {
			branch, next = "└── ", "    "
		}
		if e.isDir {
			child := n.dirs[e.name]
			*out = append(*out, fmt.Sprintf("%s%s%s/ (%s, %s)", prefix, branch, e.name, plural(child.count, "file"), humanSize(child.size)))
			if depth+1 < maxDepth {
				walkTree(child, prefix+next, depth+1, maxDepth, out)
			}
			continue
		}
		*out = append(*out, prefix+branch+e.name)
	}
	if overflow > 0 {
		*out = append(*out, fmt.Sprintf("%s└── %d more", prefix, overflow))
	}
}

func entryLines(root string, files []scannedFile) []string {
	have := map[string]bool{}
	for _, f := range files {
		have[f.rel] = true
	}

	var lines []string
	add := func(s string) { lines = append(lines, "- "+s) }

	if have["package.json"] {
		if b, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
			var pkg struct {
				Name    string            `json:"name"`
				Scripts map[string]string `json:"scripts"`
			}
			if json.Unmarshal(b, &pkg) == nil {
				var names []string
				for k := range pkg.Scripts {
					names = append(names, k)
				}
				sort.Strings(names)
				if len(names) > 6 {
					names = names[:6]
				}
				desc := "`package.json`"
				if pkg.Name != "" {
					desc += ": " + pkg.Name
				}
				if len(names) > 0 {
					desc += " (scripts: " + strings.Join(names, ", ") + ")"
				}
				add(desc)
			}
		}
	}
	if have["go.mod"] {
		if b, err := os.ReadFile(filepath.Join(root, "go.mod")); err == nil {
			for _, line := range strings.Split(string(b), "\n") {
				if strings.HasPrefix(line, "module ") {
					add("`go.mod`: " + strings.TrimSpace(strings.TrimPrefix(line, "module ")))
					break
				}
			}
		}
	}
	for _, name := range []string{"Cargo.toml", "pyproject.toml"} {
		if have[name] {
			if b, err := os.ReadFile(filepath.Join(root, name)); err == nil {
				for _, line := range strings.Split(string(b), "\n") {
					if strings.HasPrefix(strings.TrimSpace(line), "name = ") {
						add("`" + name + "`: " + strings.Trim(strings.TrimSpace(line), `name = "`))
						break
					}
				}
			}
		}
	}
	if have["Makefile"] {
		if b, err := os.ReadFile(filepath.Join(root, "Makefile")); err == nil {
			var targets []string
			for _, line := range strings.Split(string(b), "\n") {
				if line == "" || strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "#") {
					continue
				}
				if i := strings.Index(line, ":"); i > 0 && !strings.ContainsAny(line[:i], " =") {
					targets = append(targets, line[:i])
				}
			}
			if len(targets) > 8 {
				targets = targets[:8]
			}
			if len(targets) > 0 {
				add("`Makefile`: " + strings.Join(targets, ", "))
			}
		}
	}
	for _, name := range []string{"Dockerfile", "docker-compose.yml", "compose.yaml", "flake.nix"} {
		if have[name] {
			add("`" + name + "`")
		}
	}

	if len(lines) == 0 {
		lines = append(lines, "- none detected")
	}
	return lines
}

func largestLines(files []scannedFile) []string {
	sorted := make([]scannedFile, len(files))
	copy(sorted, files)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].lines != sorted[j].lines {
			return sorted[i].lines > sorted[j].lines
		}
		return sorted[i].size > sorted[j].size
	})

	var lines []string
	for i, f := range sorted {
		if i == 5 {
			break
		}
		if f.lines > 0 {
			lines = append(lines, fmt.Sprintf("- `%s` (%d lines)", f.rel, f.lines))
			continue
		}
		lines = append(lines, fmt.Sprintf("- `%s` (%s)", f.rel, humanSize(f.size)))
	}
	if len(lines) == 0 {
		lines = append(lines, "- no files")
	}
	return lines
}
