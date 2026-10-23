<!-- vistan: adr -->

# ADR: serve pages over HTTP instead of opening files

## Status

accepted — 20 Oct 2026

## Context

Pages used to be written to a file and handed to the browser as a `file://` URL. That works for reading, but it breaks anything that needs an origin: the clipboard API, relative links, and any future endpoint that reports back. It also made every run a cold start with no shared address.

The binary is installed with `go install`, so whatever it does has to work on a machine with nothing else installed.

## Options considered

- **`file://` plus a clipboard shim** — no server, but the browser treats every file as its own origin and the shim is a second binary.
- **Write the page and print a URL to an editor** — no browser, no clipboard, and the user has to open it themselves.
- **A server per page** — clean origin, but a process per document and a new port every time.

## Decision

The binary starts a small HTTP server on `127.0.0.1` and serves the generated pages from the cache directory. It re-executes itself in a new session so it outlives the command that started it, and writes a pid file so `vistan stop` can find it later.

```go
cmd := exec.Command(exe, "serve-child", "--dir", cache, "--port", port)
cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
```

## Consequences

- Pages get a real origin, so relative links work and the page can post a choice back to the server.
- One more process to track; the pid file is the only handle on it.
- The server binds to loopback with no auth, so it is not shareable and not safe to expose.
- `vistan stop` is required after a rename, because the directory is the only thing the server knows.

## Links

- [server.go](../server.go)
- [issue #3: pages disappear when the command exits](https://github.com/zovaris/vistan/issues/3)
