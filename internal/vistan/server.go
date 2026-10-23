package vistan

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func pidPath(cache string) string { return filepath.Join(cache, "server") }

func readPidFile(cache string) (int, int, bool) {
	b, err := os.ReadFile(pidPath(cache))
	if err != nil {
		return 0, 0, false
	}
	fields := strings.Fields(string(b))
	if len(fields) < 2 {
		return 0, 0, false
	}
	pid, err1 := strconv.Atoi(fields[0])
	port, err2 := strconv.Atoi(fields[1])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return pid, port, true
}

func alive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func portFree(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	l.Close()
	return true
}

func startServer(cache string, port int) (int, error) {
	if pid, known, ok := readPidFile(cache); ok && alive(pid) {
		if runningVersion(cache) == version {
			return known, nil
		}
		// A server from an older build cannot answer /_answer or serve the new page. It
		// outlives an upgrade, so replace it instead of trusting the live pid.
		syscall.Kill(pid, syscall.SIGTERM)
		for i := 0; i < 50 && alive(pid); i++ {
			time.Sleep(20 * time.Millisecond)
		}
	}
	os.Remove(pidPath(cache))

	if !portFree(port) {
		return 0, fmt.Errorf("port %d is busy; run with VISTAN_PORT=<other>", port)
	}

	exe, err := os.Executable()
	if err != nil {
		return 0, err
	}
	cmd := exec.Command(exe, "serve-child", "--dir", cache, "--port", strconv.Itoa(port))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}

	if err := os.WriteFile(pidPath(cache), []byte(fmt.Sprintf("%d %d %s\n", cmd.Process.Pid, port, version)), 0o644); err != nil {
		return 0, err
	}

	client := &http.Client{Timeout: 300 * time.Millisecond}
	url := fmt.Sprintf("http://127.0.0.1:%d/", port)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			return port, nil
		}
		time.Sleep(40 * time.Millisecond)
	}
	return port, nil
}

func runningVersion(cache string) string {
	b, err := os.ReadFile(pidPath(cache))
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(b))
	if len(fields) < 3 {
		return ""
	}
	return fields[2]
}

func serveBase(dir string) string {
	port := 8765
	if p := os.Getenv("VISTAN_PORT"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}
	if _, err := startServer(dir, port); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ""
	}
	if pid, known, ok := readPidFile(dir); ok && pid > 0 {
		port = known
	}
	return fmt.Sprintf("http://127.0.0.1:%d/", port)
}

func runServer(dir string, port int) error {
	mux := http.NewServeMux()
	mux.Handle("POST /_answer/", answerHandler(dir))
	mux.Handle("/", http.FileServer(http.Dir(dir)))
	srv := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return srv.ListenAndServe()
}

const maxAnswer = 4 << 10

type answer struct {
	Page   string    `json:"page"`
	Answer string    `json:"answer"`
	At     time.Time `json:"at"`
}

// answerHandler takes the choice a human made on a page and writes it next to the
// page, where `vistan answer <name>` reads it. The page is the only writer: a foreign
// origin is refused, so another site cannot answer on the human's behalf.
func answerHandler(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			http.Error(w, "refusing a foreign origin", http.StatusForbidden)
			return
		}
		segment := strings.TrimPrefix(r.URL.Path, "/_answer/")
		if segment == "" || strings.Contains(segment, "/") {
			http.Error(w, "no page name", http.StatusBadRequest)
			return
		}
		name := pageName(strings.TrimSuffix(segment, ".html"))

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxAnswer))
		if err != nil {
			http.Error(w, "answer too large", http.StatusRequestEntityTooLarge)
			return
		}
		var sent struct {
			Answer string `json:"answer"`
		}
		if err := json.Unmarshal(body, &sent); err != nil || strings.TrimSpace(sent.Answer) == "" {
			http.Error(w, "expected {\"answer\": \"...\"}", http.StatusBadRequest)
			return
		}

		payload, err := json.Marshal(answer{Page: name, Answer: sent.Answer, At: time.Now()})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tmp := filepath.Join(dir, "."+name+".answer.tmp")
		if err := os.WriteFile(tmp, payload, 0o644); err != nil {
			http.Error(w, "could not store the answer", http.StatusInternalServerError)
			return
		}
		if err := os.Rename(tmp, filepath.Join(dir, name+".answer.json")); err != nil {
			http.Error(w, "could not store the answer", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	}
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return u.Scheme == "http" && u.Host == r.Host
}

func stopServer(cache string, verbose bool) error {
	pid, _, ok := readPidFile(cache)
	if !ok {
		if verbose {
			fmt.Println("no server running")
		}
		return nil
	}
	if alive(pid) {
		syscall.Kill(pid, syscall.SIGTERM)
	}
	os.Remove(pidPath(cache))
	if verbose {
		fmt.Println("server stopped")
	}
	return nil
}
