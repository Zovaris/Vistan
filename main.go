package main

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

const version = "0.1.0"

func usage(w io.Writer) {
	fmt.Fprintf(w, `usage: vistan <file.md> | serve <file.md> | stop | new <template> | templates

  serve        build the page, start a local server and open it
  stop         stop the local server
  new          print a template to stdout
  templates    list the templates
  --template   override the template declared in the document

Templates: plan, summary, selection, map, diagram.
Declare one in the document with  <!-- vistan: map -->  or pass --template.

Env: VISTAN_PORT (default 8765), VISTAN_NO_OPEN=1, VISTAN_CACHE.
`)
}

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	command := "show"
	if len(args) > 0 {
		switch args[0] {
		case "serve", "stop", "new", "templates", "serve-child":
			command = args[0]
			args = args[1:]
		case "-h", "--help", "help":
			usage(os.Stderr)
			return 2
		}
	}

	switch command {
	case "stop":
		if err := stopServer(cacheDir()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		return 0
	case "templates":
		for _, t := range templateList {
			fmt.Println(t.Name)
		}
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
	case "serve-child":
		return serveChild(args)
	}

	return show(command, args)
}

func cacheDir() string {
	if dir := os.Getenv("VISTAN_CACHE"); dir != "" {
		return dir
	}
	return filepath.Join(os.TempDir(), "vistan")
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
	var source, name string

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
		name = strings.TrimSuffix(filepath.Base(src), ".md")
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

	cache := cacheDir()
	if err := os.MkdirAll(cache, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	out := filepath.Join(cache, name+".html")
	if err := os.WriteFile(out, []byte(renderHTML(doc, tpl, source)), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

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
