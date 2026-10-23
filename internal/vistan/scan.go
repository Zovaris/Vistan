package vistan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"math"
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

	doc := &Doc{Title: filepath.Base(abs), Artifact: treemapSVG(files, filepath.Base(abs))}
	doc.Sections = []Section{
		{Name: "Overview", Body: overviewLines(files, stats, source)},
	}
	if lines := gitLines(abs); len(lines) > 0 {
		doc.Sections = append(doc.Sections, Section{Name: "Git", Body: lines})
	}
	doc.Sections = append(doc.Sections,
		Section{Name: "Tree", Body: treeLines(filepath.Base(abs), files, depth)},
		Section{Name: "Entry points", Body: entryLines(abs, files)},
		Section{Name: "Largest files", Body: largestLines(files)},
	)
	return doc, nil
}

func gitLines(root string) []string {
	branch, err := gitOutput(root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil
	}
	branch = strings.TrimSpace(branch)
	if branch == "HEAD" {
		if short, err := gitOutput(root, "rev-parse", "--short", "HEAD"); err == nil {
			branch = "detached at " + short
		}
	}
	lines := []string{fmt.Sprintf("- `branch` `%s`", branch)}

	if head, err := gitOutput(root, "log", "-1", "--format=%h%x09%s%x09%cr"); err == nil {
		if parts := strings.Split(strings.TrimSpace(head), "\t"); len(parts) == 3 {
			lines = append(lines, fmt.Sprintf("- `commit` `%s` %s · %s", parts[0], parts[1], parts[2]))
		}
	}

	status, err := gitOutput(root, "status", "--porcelain")
	if err != nil {
		return lines
	}
	var changed []string
	for _, line := range strings.Split(status, "\n") {
		if len(line) < 4 {
			continue
		}
		code := strings.TrimSpace(line[:2])
		if code == "" {
			continue
		}
		if strings.HasPrefix(code, "??") {
			code = "?"
		}
		changed = append(changed, fmt.Sprintf("%c %s", code[0], strings.TrimSpace(line[3:])))
	}
	if len(changed) == 0 {
		return append(lines, "- working tree clean")
	}

	lines = append(lines, fmt.Sprintf("- %d file%s changed", len(changed), pluralSuffix(len(changed))), "", "```")
	for i, f := range changed {
		if i == 12 {
			lines = append(lines, fmt.Sprintf("… and %d more", len(changed)-12))
			break
		}
		lines = append(lines, f)
	}
	return append(lines, "```")
}

func gitOutput(root string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
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
	// A partial tree is still worth a page: an unreadable directory skips its subtree.
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
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
						value := strings.TrimPrefix(strings.TrimSpace(line), "name = ")
						add("`" + name + "`: " + strings.Trim(value, `"`))
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

type tmNode struct {
	name string
	size int64
	kids []*tmNode
}

func buildTM(root string, files []scannedFile) *tmNode {
	rootNode := &tmNode{name: root}
	index := map[string]*tmNode{"": rootNode}
	for _, f := range files {
		parts := strings.Split(filepath.ToSlash(f.rel), "/")
		parent := rootNode
		path := ""
		for _, part := range parts[:len(parts)-1] {
			if path == "" {
				path = part
			} else {
				path += "/" + part
			}
			child := index[path]
			if child == nil {
				child = &tmNode{name: part}
				index[path] = child
				parent.kids = append(parent.kids, child)
			}
			parent = child
		}
		parent.kids = append(parent.kids, &tmNode{name: parts[len(parts)-1], size: f.size})
	}

	var measure func(n *tmNode) int64
	measure = func(n *tmNode) int64 {
		for _, kid := range n.kids {
			n.size += measure(kid)
		}
		return n.size
	}
	measure(rootNode)
	return rootNode
}

type tmRect struct{ x, y, w, h float64 }

const tmHues = 6

func treemapSVG(files []scannedFile, root string) string {
	tree := buildTM(root, files)
	if tree.size <= 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<svg class="tm-map" viewBox="0 0 720 300" role="img" aria-label="Where the bytes are">` + "\n")
	placeTM(tree, tmRect{0, 0, 720, 300}, 0, 0, &b)
	b.WriteString("</svg>\n")
	b.WriteString(treemapRows(tree))
	b.WriteString(`<p class="tm-cap" aria-hidden="true">Hover a tile to see its name and size</p>` + "\n")
	b.WriteString(treemapHover)
	return b.String()
}

// treemapRows is the narrow-screen stand-in for the treemap: the same top-level
// sizes as text and a bar, so no label ever scales below the body size.
func treemapRows(root *tmNode) string {
	kids := make([]*tmNode, len(root.kids))
	copy(kids, root.kids)
	if len(kids) == 0 {
		return ""
	}
	sort.Slice(kids, func(i, j int) bool {
		if kids[i].size != kids[j].size {
			return kids[i].size > kids[j].size
		}
		return kids[i].name < kids[j].name
	})
	largest := kids[0].size
	if largest <= 0 {
		return ""
	}

	const maxRows = 7
	shown := kids
	var rest *tmNode
	if len(kids) > maxRows {
		rest = &tmNode{name: fmt.Sprintf("%d more", len(kids)-maxRows+1)}
		for _, kid := range kids[maxRows-1:] {
			rest.size += kid.size
		}
		shown = kids[:maxRows-1]
	}

	var b strings.Builder
	b.WriteString("<ul class=\"tm-rows\">\n")
	row := func(class, name string, size int64) {
		fmt.Fprintf(&b, `<li%s><span class="nm">%s</span><span class="bar"><i style="width:%.1f%%"></i></span><span class="sz">%s</span></li>`+"\n",
			class, html.EscapeString(name), float64(size)/float64(largest)*100, humanSize(size))
	}
	for _, kid := range shown {
		name := kid.name
		if len(kid.kids) > 0 {
			name += "/"
		}
		row("", name, kid.size)
	}
	if rest != nil {
		row(` class="more"`, rest.name, rest.size)
	}
	b.WriteString("</ul>\n")
	return b.String()
}

// treemapHover names the tile under the cursor in one line below the map, so a
// tile too small for its own text needs no text at all.
const treemapHover = `<script>
(() => {
  const svg = document.querySelector('.art svg');
  if (!svg) return;
  const cap = svg.parentElement.querySelector('.tm-cap');
  const idle = cap.textContent;
  let hot = null;
  svg.addEventListener('pointerover', e => {
    const g = e.target.closest('g.tm');
    if (!g || g === hot) return;
    if (hot) hot.classList.remove('hot');
    hot = g;
    hot.classList.add('hot');
    const t = g.querySelector('title');
    if (t) cap.textContent = t.textContent;
  });
  svg.addEventListener('pointerleave', () => {
    if (hot) hot.classList.remove('hot');
    hot = null;
    cap.textContent = idle;
  });
})();
</script>`

func placeTM(n *tmNode, r tmRect, depth, hue int, b *strings.Builder) {
	if depth >= 2 || len(n.kids) == 0 || r.w < 10 || r.h < 10 {
		return
	}

	kids := make([]*tmNode, len(n.kids))
	copy(kids, n.kids)
	sort.Slice(kids, func(i, j int) bool {
		if kids[i].size != kids[j].size {
			return kids[i].size > kids[j].size
		}
		return kids[i].name < kids[j].name
	})
	if len(kids) > 16 {
		rest := &tmNode{name: fmt.Sprintf("%d more", len(kids)-15)}
		for _, kid := range kids[15:] {
			rest.size += kid.size
		}
		kids = append(kids[:15], rest)
	}

	for i, spot := range squarify(kids, r) {
		color := hue
		if depth == 0 {
			color = i % tmHues
		}
		shade := "tm-d0"
		if depth > 0 {
			shade = "tm-d1"
		}

		rect := spot.rect
		labeled := rect.w >= 64 && rect.h >= 20
		fmt.Fprintf(b, `<g class="tm tm-c%d %s"><title>%s · %s</title>`, color, shade,
			html.EscapeString(spot.node.name), humanSize(spot.node.size))
		fmt.Fprintf(b, `<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" rx="2"/>`,
			rect.x+1, rect.y+1, math.Max(rect.w-2, 0), math.Max(rect.h-2, 0))
		if labeled {
			fmt.Fprintf(b, `<text x="%.1f" y="%.1f" font-size="10">%s</text>`,
				rect.x+7, rect.y+15, html.EscapeString(spot.node.name))
			if rect.h >= 34 {
				fmt.Fprintf(b, `<text class="sub" x="%.1f" y="%.1f" font-size="9">%s</text>`,
					rect.x+7, rect.y+28, humanSize(spot.node.size))
			}
		}
		b.WriteString("</g>\n")

		top := 3.0
		if labeled {
			top = 21
			if rect.h >= 34 {
				top = 34
			}
		}
		placeTM(spot.node, tmRect{rect.x + 2, rect.y + top, rect.w - 4, rect.h - top - 2}, depth+1, color, b)
	}
}

type tmSpot struct {
	node *tmNode
	rect tmRect
}

func squarify(nodes []*tmNode, r tmRect) []tmSpot {
	total := 0.0
	for _, n := range nodes {
		total += float64(n.size)
	}
	if total <= 0 || r.w <= 0 || r.h <= 0 {
		return nil
	}

	scale := r.w * r.h / total
	areas := make([]float64, len(nodes))
	for i, n := range nodes {
		areas[i] = float64(n.size) * scale
	}

	var out []tmSpot
	x, y, w, h := r.x, r.y, r.w, r.h
	for i := 0; i < len(nodes); {
		row := []int{i}
		i++
		for i < len(nodes) && worse(append(row, i), areas, math.Min(w, h)) <= worse(row, areas, math.Min(w, h)) {
			row = append(row, i)
			i++
		}

		sum := 0.0
		for _, k := range row {
			sum += areas[k]
		}
		if w >= h {
			col := sum / h
			oy := y
			for _, k := range row {
				ih := areas[k] / col
				out = append(out, tmSpot{nodes[k], tmRect{x, oy, col, ih}})
				oy += ih
			}
			x += col
			w -= col
			continue
		}
		rowH := sum / w
		ox := x
		for _, k := range row {
			iw := areas[k] / rowH
			out = append(out, tmSpot{nodes[k], tmRect{ox, y, iw, rowH}})
			ox += iw
		}
		y += rowH
		h -= rowH
	}
	return out
}

func worse(row []int, areas []float64, side float64) float64 {
	if len(row) == 0 || side <= 0 {
		return math.Inf(1)
	}
	sum, maxArea, minArea := 0.0, 0.0, math.Inf(1)
	for _, k := range row {
		sum += areas[k]
		maxArea = math.Max(maxArea, areas[k])
		minArea = math.Min(minArea, areas[k])
	}
	if sum == 0 || minArea == 0 {
		return math.Inf(1)
	}
	return math.Max(side*side*maxArea/(sum*sum), sum*sum/(side*side*minArea))
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
