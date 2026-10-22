<!-- vistan: plan -->

# vistan: a page a human reads

## Decisions

- Documents are Markdown with fixed sections; the tool never invents content.
- One Go binary with `net/http` and `embed`: no runtime, no dependencies, `go install` and done.
- Templates live as embedded `.md` files, so the binary installs its own content.
- `serve` re-executes itself in a new session, so the page outlives the command that opened it.
- `map` draws its own SVG treemap instead of shipping an image or a chart library.

## Rejected

- Bash plus Python: the "binary" would ask for an interpreter on a clean machine.
- A JS renderer: a runtime and a package tree for one page of text.
- Mermaid: a network dependency for diagrams that ASCII already draws.
- `file://` pages: no origin, so the clipboard API and relative links break.

## Steps

- [x] Parse sections, lists, images and subheads with one parser.
- [x] Serve pages from a detached local server.
- [x] Draw a treemap for `map` and index every generated page.
- [ ] Decide where the generated pages live.

## Files

```changes
+ doc.go
+ render.go
+ scan.go
+ index.go
~ main.go  wires map, index and stop
~ INSTRUCTIONS.md
- vistan  the bash script, replaced by the binary
```

## Verification

- [x] `go test ./...`: 19 checks, green.
- [x] `go vet` and `gofmt -l` clean.
- [x] The seven example pages answer 200 in the browser.
- [ ] Try the copy button in Safari, not just Chromium.

> [!WARN] The server has no auth
> It binds to `127.0.0.1` and serves whatever is in the cache. Do not put it behind a public tunnel: the pages contain code and paths from your project.

## Your move

- Decide whether generated pages belong in `.vistan/` inside the project or in the cache.
- Decide if `diagram` should stay ASCII or accept a drawn format.
