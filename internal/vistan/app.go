package vistan

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const version = "0.2.0"

func usage(w io.Writer) {
	fmt.Fprintf(w, `usage: vistan <file.md> | serve <file.md> | stop | new <template> | templates

  serve        build the page, start a local server and open it
  map [path]   scan a directory and render it as a map (default: .)
  docs [path]  index the ADRs and specs under a directory (default: .)
  history      list the saved pages and serve them
  index        same as history
  history --json  print the saved pages as JSON, no server
  open <name>  open a saved page by name or prefix
  answer <name>  print the answer the human sent from a page (exit 1 if none yet)
  rerender <name>  rebuild a saved page from its stored source
  rerender --all   rebuild every saved page that has a stored source
  rm <name>    delete saved pages
  stop         stop the local server
  clean        stop the server and delete the saved pages
  uninstall    clean, then delete the binary
  new          print a template to stdout
  templates    list the templates
  version      print the version of the binary
  brief        print the rules for the agent that writes the pages; append it to
               that agent's AGENTS.md or CLAUDE.md to teach it vistan
  --template   override the template declared in the document

Templates: plan, summary, selection, guide, review, map, diagram, graph, adr,
spec, changelog.
Declare one in the document with  <!-- vistan: map -->  or pass --template.

Env: VISTAN_PORT (default 8765), VISTAN_NO_OPEN=1, VISTAN_CACHE, VISTAN_KEEP
(default 50 pages; 0 keeps all), VISTAN_KEEP_DAYS (default 30; 0 keeps any age).

Pages are kept between runs, newest first, in the user state directory:
~/.local/state/vistan, ~/Library/Application Support/vistan or %%AppData%%\vistan.
`)
}

// splitFlags turns --name=value into two arguments, so the flag loops below can keep
// reading the value from the argument that follows.
func splitFlags(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if strings.HasPrefix(a, "--") {
			if name, value, ok := strings.Cut(a, "="); ok {
				out = append(out, name, value)
				continue
			}
		}
		out = append(out, a)
	}
	return out
}

func Run(args []string) int {
	args = splitFlags(args)
	for i, a := range args {
		if a == "-h" || a == "--help" || (i == 0 && a == "help") {
			usage(os.Stdout)
			return 0
		}
		if a == "--version" || (i == 0 && a == "version") {
			fmt.Printf("vistan %s\n", version)
			return 0
		}
	}

	command := "show"
	if len(args) > 0 {
		switch args[0] {
		case "serve", "stop", "clean", "uninstall", "new", "templates", "map", "docs", "index", "history", "open", "answer", "rerender", "rm", "brief", "serve-child":
			command = args[0]
			args = args[1:]
		}
	}

	switch command {
	case "stop":
		if err := stopServer(stateDir(), true); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		if legacy := filepath.Join(os.TempDir(), "vistan"); legacy != stateDir() {
			_ = stopServer(legacy, false)
		}
		return 0
	case "clean":
		return runClean()
	case "uninstall":
		return runUninstall()
	case "templates":
		for _, t := range templateList {
			fmt.Println(t.Name)
		}
		return 0
	case "brief":
		fmt.Println(agentBrief)
		usage(os.Stdout)
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
		return runIndex(args)
	case "open":
		return runOpen(args)
	case "answer":
		return runAnswer(args)
	case "rerender":
		return runRerender(args)
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

func runClean() int {
	dir := stateDir()
	if err := stopServer(dir, false); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if legacy := filepath.Join(os.TempDir(), "vistan"); legacy != dir {
		_ = stopServer(legacy, false)
		if os.Getenv("VISTAN_CACHE") == "" {
			_ = os.RemoveAll(legacy)
		}
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fmt.Println("nothing to remove")
		return 0
	}
	if err := removeState(dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	fmt.Printf("removed %s\n", dir)
	return 0
}

func runUninstall() int {
	if code := runClean(); code != 0 {
		return code
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if err := os.Remove(exe); err != nil {
		fmt.Fprintf(os.Stderr, "could not delete the binary: %v\nremove it by hand: %s\n", err, exe)
		return 2
	}
	fmt.Printf("removed %s\n", exe)
	return 0
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

	return serveDoc(doc, tpl, source, name, baseDir, lines)
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
	return serveDoc(doc, tpl, abs, pageName(filepath.Base(abs)), "", nil)
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

func writePage(dir, name, content string, lines []string) error {
	if err := os.WriteFile(filepath.Join(dir, name+".html"), []byte(content), 0o644); err != nil {
		return err
	}
	if len(lines) == 0 {
		return nil
	}
	source := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(filepath.Join(dir, name+".md"), []byte(source), 0o644)
}

func serveDoc(doc *Doc, tpl Template, source, name, baseDir string, lines []string) int {
	cache := stateDir()
	if err := os.MkdirAll(cache, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if err := writePage(cache, name, renderHTML(doc, tpl, source, baseDir), lines); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	// A page that is rendered again is a new question: an answer from the last time it
	// was shown would be read as the answer to this one.
	os.Remove(filepath.Join(cache, name+".answer.json"))
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
