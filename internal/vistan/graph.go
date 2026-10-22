package vistan

import (
	"fmt"
	"html"
	"strings"
)

type graphNode struct {
	name  string
	label string
}

type graphEdge struct {
	from  string
	to    string
	label string
}

func splitEdge(line string) (string, string, bool) {
	for _, arrow := range []string{"-->", "->", "=>", "→", "—>"} {
		if i := strings.Index(line, arrow); i > 0 {
			return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+len(arrow):]), true
		}
	}
	return "", "", false
}

func splitNodeLabel(text string) (string, string) {
	text = strings.TrimSpace(text)
	if strings.HasSuffix(text, "]") {
		if i := strings.LastIndex(text, "["); i > 0 {
			return strings.TrimSpace(text[:i]), strings.TrimSpace(strings.TrimSuffix(text[i+1:], "]"))
		}
	}
	return text, ""
}

func parseGraph(lines []string) ([]graphNode, []graphEdge) {
	var nodes []graphNode
	var edges []graphEdge
	index := map[string]int{}

	add := func(name, label string) string {
		name = strings.TrimSpace(strings.Trim(name, "`"))
		if name == "" {
			return ""
		}
		if i, ok := index[name]; ok {
			if label != "" {
				nodes[i].label = label
			}
			return name
		}
		if label == "" {
			label = strings.ReplaceAll(name, "_", " ")
		}
		index[name] = len(nodes)
		nodes = append(nodes, graphNode{name: name, label: label})
		return name
	}

	for _, line := range lines {
		text := strings.TrimSpace(line)
		if text == "" || strings.HasPrefix(text, "```") || strings.HasPrefix(text, "#") {
			continue
		}
		label := ""
		if i := strings.Index(text, " : "); i > 0 {
			label = strings.TrimSpace(text[i+3:])
			text = strings.TrimSpace(text[:i])
		}

		from, to, ok := splitEdge(text)
		if !ok {
			name, nodeLabel := splitNodeLabel(text)
			add(name, nodeLabel)
			continue
		}
		fromName, fromLabel := splitNodeLabel(from)
		toName, toLabel := splitNodeLabel(to)
		fromName = add(fromName, fromLabel)
		toName = add(toName, toLabel)
		if fromName == "" || toName == "" {
			continue
		}
		edges = append(edges, graphEdge{from: fromName, to: toName, label: label})
	}
	return nodes, edges
}

type graphBox struct {
	node graphNode
	x, y float64
	w, h float64
}

func graphSVG(lines []string) string {
	nodes, edges := parseGraph(lines)
	if len(nodes) == 0 {
		return ""
	}

	level := graphLevels(nodes, edges)

	const boxH, gapX, rowGap = 30.0, 30.0, 44.0
	rows := map[int][]graphNode{}
	deepest := 0
	for _, node := range nodes {
		depth := level[node.name]
		if depth > deepest {
			deepest = depth
		}
		rows[depth] = append(rows[depth], node)
	}

	width := func(label string) float64 {
		w := 22 + 7.1*float64(len([]rune(label)))
		if w < 74 {
			w = 74
		}
		if w > 196 {
			w = 196
		}
		return w
	}

	total := 0.0
	for depth := 0; depth <= deepest; depth++ {
		row := rows[depth]
		sortNodes(row)
		sum := 0.0
		for _, node := range row {
			sum += width(node.label)
		}
		if len(row) > 1 {
			sum += gapX * float64(len(row)-1)
		}
		if sum > total {
			total = sum
		}
	}
	if total < 640 {
		total = 640
	}
	height := float64(deepest+1)*boxH + float64(deepest)*rowGap + 8

	boxes := map[string]graphBox{}
	for depth := 0; depth <= deepest; depth++ {
		row := rows[depth]
		rowWidth := 0.0
		for _, node := range row {
			rowWidth += width(node.label)
		}
		if len(row) > 1 {
			rowWidth += gapX * float64(len(row)-1)
		}
		x := (total - rowWidth) / 2
		y := 4 + float64(depth)*(boxH+rowGap)
		for _, node := range row {
			w := width(node.label)
			boxes[node.name] = graphBox{node: node, x: x, y: y, w: w, h: boxH}
			x += w + gapX
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="graph" viewBox="0 0 %.0f %.0f" role="img" aria-label="%s">`+"\n",
		total, height, html.EscapeString(graphLabel(nodes)))
	b.WriteString(`<defs><marker id="g-arrow" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0,0 L8,4 L0,8 z" class="arrowhead"/></marker></defs>` + "\n")

	for _, edge := range edges {
		from, okFrom := boxes[edge.from]
		to, okTo := boxes[edge.to]
		if !okFrom || !okTo {
			continue
		}
		x1, y1 := from.x+from.w/2, from.y+from.h
		x2, y2 := to.x+to.w/2, to.y
		var path string
		lane := 0.0
		back := false
		if y2 > y1 {
			mid := (y1 + y2) / 2
			path = fmt.Sprintf("M%.1f,%.1f C%.1f,%.1f %.1f,%.1f %.1f,%.1f", x1, y1, x1, mid, x2, mid, x2, y2-3)
		} else {
			lane = total - 12
			path = fmt.Sprintf("M%.1f,%.1f H%.1f V%.1f H%.1f",
				from.x+from.w, y1-from.h/2, lane, y2+to.h/2, to.x+to.w)
			back = true
		}
		class := "edge"
		if back {
			class = "edge back"
		}
		fmt.Fprintf(&b, `<path class="%s" d="%s" marker-end="url(#g-arrow)"><title>%s → %s</title></path>`+"\n",
			class, path, html.EscapeString(edge.from), html.EscapeString(edge.to))
		if edge.label != "" {
			labelX, labelY := (x1+x2)/2, (y1+y2)/2-4
			anchor := "middle"
			if back {
				labelX, labelY, anchor = lane-8, (y1+y2)/2, "end"
			}
			fmt.Fprintf(&b, `<text class="edge-label" x="%.1f" y="%.1f" text-anchor="%s">%s</text>`+"\n",
				labelX, labelY, anchor, html.EscapeString(edge.label))
		}
	}

	for depth := 0; depth <= deepest; depth++ {
		for _, node := range rows[depth] {
			box := boxes[node.name]
			fmt.Fprintf(&b, `<g class="node"><title>%s</title><rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="4"/>`,
				html.EscapeString(node.name), box.x, box.y, box.w, box.h)
			fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="middle" dominant-baseline="middle">%s</text></g>`+"\n",
				box.x+box.w/2, box.y+box.h/2+1, html.EscapeString(node.label))
		}
	}

	b.WriteString("</svg>")
	return b.String()
}

func graphLevels(nodes []graphNode, edges []graphEdge) map[string]int {
	incoming := map[string]int{}
	for _, edge := range edges {
		incoming[edge.to]++
	}

	level := map[string]int{}
	seen := map[string]bool{}
	var queue []string
	seed := func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		queue = append(queue, name)
	}

	for _, node := range nodes {
		if incoming[node.name] == 0 {
			seed(node.name)
		}
	}
	for i := 0; i < len(nodes); i++ {
		if !seen[nodes[i].name] {
			seed(nodes[i].name)
		}
		for j := 0; j < len(queue); j++ {
			current := queue[j]
			for _, edge := range edges {
				if edge.from != current || seen[edge.to] {
					continue
				}
				seen[edge.to] = true
				level[edge.to] = level[current] + 1
				queue = append(queue, edge.to)
			}
		}
	}
	return level
}

func sortNodes(nodes []graphNode) {
	for i := 1; i < len(nodes); i++ {
		for j := i; j > 0 && nodes[j].name < nodes[j-1].name; j-- {
			nodes[j], nodes[j-1] = nodes[j-1], nodes[j]
		}
	}
}

func graphLabel(nodes []graphNode) string {
	if len(nodes) == 1 {
		return nodes[0].label
	}
	return fmt.Sprintf("Graph of %d nodes", len(nodes))
}
