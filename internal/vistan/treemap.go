package vistan

import (
	"fmt"
	"html"
	"math"
	"path/filepath"
	"sort"
	"strings"
)

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
