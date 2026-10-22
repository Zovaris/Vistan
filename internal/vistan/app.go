package vistan

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const version = "0.1.0"

func usage(w io.Writer) {
	fmt.Fprintf(w, `usage: vistan <file.md> | serve <file.md> | stop | new <template> | templates

  serve        build the page, start a local server and open it
  map [path]   scan a directory and render it as a map (default: .)
  docs [path]  index the ADRs and specs under a directory (default: .)
  history      list the saved pages and serve them (same as index)
  open <name>  open a saved page by name or prefix
  rm <name>    delete saved pages
  stop         stop the local server
  new          print a template to stdout
  templates    list the templates
  brief        print the brief for the agent that writes the pages
  --template   override the template declared in the document

Templates: plan, summary, selection, review, map, diagram, graph, adr, spec,
changelog.
Declare one in the document with  <!-- vistan: map -->  or pass --template.

Env: VISTAN_PORT (default 8765), VISTAN_NO_OPEN=1, VISTAN_CACHE, VISTAN_KEEP
(default 50 pages; 0 keeps all).

Pages are kept between runs, newest first, in the user state directory:
~/.local/state/vistan, ~/Library/Application Support/vistan or %%AppData%%\vistan.
`)
}

func Run(args []string) int {
	command := "show"
	if len(args) > 0 {
		switch args[0] {
		case "serve", "stop", "new", "templates", "map", "docs", "index", "history", "open", "rm", "brief", "serve-child":
			command = args[0]
			args = args[1:]
		case "-h", "--help", "help":
			usage(os.Stderr)
			return 2
		}
	}

	switch command {
	case "stop":
		if err := stopServer(stateDir(), true); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		if legacy := filepath.Join(os.TempDir(), "vistan"); legacy != stateDir() {
			stopServer(legacy, false)
		}
		return 0
	case "templates":
		for _, t := range templateList {
			fmt.Println(t.Name)
		}
		return 0
	case "brief":
		fmt.Print(agentBrief)
		return 0
	case "new":
		if len(args) < 1 {
			usage(os.Stderr)
			return 2
		}
		text, ok := templateText(args[0])
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown template: %s\n", args[0])
			return 2
		}
		fmt.Print(text)
		return 0
	case "index", "history":
		return runIndex()
	case "open":
		return runOpen(args)
	case "rm":
		return runRm(args)
	case "docs":
		return runDocs(args)
	case "map":
		return runMap(args)
	case "serve-child":
		return serveChild(args)
	}

	return show(command, args)
}

func stateDir() string {
	if dir := os.Getenv("VISTAN_CACHE"); dir != "" {
		return dir
	}
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "vistan")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "vistan")
	}
	if runtime.GOOS == "windows" {
		if base, err := os.UserConfigDir(); err == nil {
			return filepath.Join(base, "vistan")
		}
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "vistan")
	}
	return filepath.Join(home, ".local", "state", "vistan")
}

func keepLimit() int {
	limit := 50
	if v := os.Getenv("VISTAN_KEEP"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	return limit
}

func keepPages(dir string) {
	limit := keepLimit()
	if limit <= 0 {
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	type page struct {
		name string
		mod  time.Time
	}
	var pages []page
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".html") || isIndexName(name) {
			continue
		}
		if info, err := entry.Info(); err == nil {
			pages = append(pages, page{name, info.ModTime()})
		}
	}
	if len(pages) <= limit {
		return
	}

	sort.Slice(pages, func(i, j int) bool { return pages[i].mod.After(pages[j].mod) })
	for _, p := range pages[limit:] {
		os.Remove(filepath.Join(dir, p.name))
	}
}

func serveChild(args []string) int {
	dir, port := "", 8765
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dir":
			if i+1 < len(args) {
				dir = args[i+1]
				i++
			}
		case "--port":
			if i+1 < len(args) {
				port, _ = strconv.Atoi(args[i+1])
				i++
			}
		}
	}
	if dir == "" {
		fmt.Fprintln(os.Stderr, "--dir is required")
		return 2
	}
	if err := runServer(dir, port); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	return 0
}

func show(command string, args []string) int {
	templateName := ""
	src := ""

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--template":
			if i+1 >= len(args) {
				usage(os.Stderr)
				return 2
			}
			templateName = args[i+1]
			i++
		case strings.HasPrefix(args[i], "-") && args[i] != "-":
			fmt.Fprintf(os.Stderr, "unknown option: %s\n", args[i])
			return 2
		default:
			src = args[i]
		}
	}

	if src == "" {
		usage(os.Stderr)
		return 2
	}

	var lines []string
	var source, name, baseDir string

	if src == "-" {
		if err := readLines(os.Stdin, &lines); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		source, name = "(stdin)", "document"
	} else {
		f, err := os.Open(src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "not found: %s\n", src)
			return 2
		}
		defer f.Close()
		if err := readLines(f, &lines); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		source = src
		name = pageName(strings.TrimSuffix(filepath.Base(src), ".md"))
		baseDir = filepath.Dir(src)
	}

	if templateName == "" {
		templateName = DeclaredTemplate(lines)
	}
	if templateName == "" {
		templateName = "plan"
	}
	tpl, ok := lookupTemplate(templateName)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown template: %s\n", templateName)
		return 2
	}

	doc := Parse(lines)

	if command == "show" {
		color := os.Getenv("NO_COLOR") == "" && isTerminal(os.Stdout)
		return renderTerminal(doc, tpl, source, color, os.Stdout)
	}

	return serveDoc(doc, tpl, source, name, baseDir)
}

func runMap(args []string) int {
	path := "."
	depth := 2
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--depth":
			if i+1 >= len(args) {
				usage(os.Stderr)
				return 2
			}
			if n, err := strconv.Atoi(args[i+1]); err == nil {
				depth = n
			}
			i++
		default:
			if strings.HasPrefix(args[i], "-") && args[i] != "-" {
				fmt.Fprintf(os.Stderr, "unknown option: %s\n", args[i])
				return 2
			}
			path = args[i]
		}
	}
	if depth < 1 {
		depth = 1
	}

	doc, err := scanProject(path, depth)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	tpl, _ := lookupTemplate("map")
	abs, err := filepath.Abs(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	return serveDoc(doc, tpl, path, pageName(filepath.Base(abs)), "")
}

func pageName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "document"
	}
	return out
}

func serveDoc(doc *Doc, tpl Template, source, name, baseDir string) int {
	cache := stateDir()
	if err := os.MkdirAll(cache, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	out := filepath.Join(cache, name+".html")
	if err := os.WriteFile(out, []byte(renderHTML(doc, tpl, source, baseDir)), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	keepPages(cache)
	refreshIndex(cache)

	port := 8765
	if p := os.Getenv("VISTAN_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}
	if _, err := startServer(cache, port); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if pid, known, ok := readPidFile(cache); ok && pid > 0 {
		port = known
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/%s.html", port, name)
	fmt.Println(url)

	if os.Getenv("VISTAN_NO_OPEN") == "" {
		openBrowser(url)
	}
	return 0
}

func readLines(r io.Reader, out *[]string) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		*out = append(*out, scanner.Text())
	}
	return scanner.Err()
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
