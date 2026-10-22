package main

import (
	"fmt"
	"net"
	"net/http"
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
	if len(fields) != 2 {
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
		return known, nil
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

	if err := os.WriteFile(pidPath(cache), []byte(fmt.Sprintf("%d %d\n", cmd.Process.Pid, port)), 0o644); err != nil {
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

func runServer(dir string, port int) error {
	srv := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", port),
		Handler:           http.FileServer(http.Dir(dir)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return srv.ListenAndServe()
}

func stopServer(cache string) error {
	pid, _, ok := readPidFile(cache)
	if !ok {
		fmt.Println("no server running")
		return nil
	}
	if alive(pid) {
		syscall.Kill(pid, syscall.SIGTERM)
	}
	os.Remove(pidPath(cache))
	fmt.Println("server stopped")
	return nil
}
