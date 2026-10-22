<!-- vistan: plan -->

# vistan: a page a human reads

## Decisions

- Documents are Markdown with fixed sections; the tool never invents content.
- One Go binary with `net/http` and `embed`: no runtime, no dependencies, `go install` and done.
- Templates live as embedded `.md` files, so the binary installs its own content.
- `serve` re-executes itself in a new session, so the page outlives the command that opened it.
- `map` draws its own SVG treemap instead of shipping an image or a chart library.
- `graph` lays out an edge list as inline SVG, and `docs` builds the decision index from the files themselves.

## Rejected

- Bash plus Python: the "binary" would ask for an interpreter on a clean machine.
- A JS renderer: a runtime and a package tree for one page of text.
- Mermaid: a network dependency for diagrams that ASCII already draws.
- `file://` pages: no origin, so the clipboard API and relative links break.

## Steps

- [x] Parse sections, lists, images and subheads with one parser.
- [x] Serve pages from a detached local server.
- [x] Draw a treemap for `map` and index every generated page.
- [x] Keep the last N pages in the user state directory, pruned after each write.
- [x] Group the history by project and add `open` and `rm`.
- [x] Draw an edge list as a graph, and index a folder of ADRs and specs.

## Files

```changes
+ doc.go
+ render.go
+ scan.go
+ graph.go  the edge list to SVG
+ docs.go  the ADR and spec index
+ index.go  history, open and rm
~ main.go  wires map, docs, history and stop
~ INSTRUCTIONS.md
- vistan  the bash script, replaced by the binary
```

## Verification

- [x] `go test ./...`: 40 checks, green.
- [x] `go vet` and `gofmt -l` clean.
- [x] The ten example pages answer 200 in the browser.
- [ ] Try the copy button in Safari, not just Chromium.

> [!WARN] The server has no auth
> It binds to `127.0.0.1` and serves whatever is in the cache. Do not put it behind a public tunnel: the pages contain code and paths from your project.

## Your move

- Decide whether `graph` should read links between ADRs, so the decision log draws its own supersession arrows.
- Decide if retention should be per project, so a noisy repository cannot evict another one's pages.
