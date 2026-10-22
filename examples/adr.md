<!-- vistan: adr -->

# ADR: serve pages over HTTP instead of opening files

## Status

accepted

## Context

Pages used to be written to a file and handed to the browser as a `file://` URL. That works for reading, but it breaks anything that needs an origin: the clipboard API, relative links, and any future endpoint that reports back. It also made every run a cold start with no shared address.

## Decision

The binary starts a small HTTP server on `127.0.0.1` and serves the generated pages from the cache directory. It re-executes itself in a new session so it outlives the command that started it, and writes a pid file so `vistan stop` can find it later.

## Consequences

- Pages get a real origin, so copy-to-clipboard and relative links behave.
- One more process to track; the pid file is the only handle on it.
- The server binds to loopback with no auth, so it is not shareable and not safe to expose.
