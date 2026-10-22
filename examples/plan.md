<!-- vistan: plan -->

# vistan: a page a human reads

## Decisions

- Documents are Markdown with fixed sections; the tool never invents content.
- One Go binary with `net/http` and `embed`: no runtime, no dependencies, `go install` and done.
- Templates live as embedded `.md` files, so the binary installs its own content and the agent only copies one.
- `serve` re-executes itself in a new session, so the page outlives the command that opened it.
- `map` draws its own SVG treemap instead of shipping an image or a chart library.

## Rejected

- Bash plus Python: the "binary" would ask for an interpreter on a clean machine.
- A JS renderer: a runtime and a package tree for one page of text.
- Mermaid: a network dependency for diagrams that ASCII already draws.
- `file://` pages: no origin, so the clipboard API and relative links break.

## Verification

- `go test ./...` covers the parser, both renderers, escaping, the treemap and the templates.
- `go vet` and `gofmt -l` clean.
- Every example page answers 200, and the pages were read in a browser, light and dark.
- The selection page: a real click fills the answer, the copy button copies it.

## Your move

- Decide whether the generated pages belong in `.vistan/` inside the project or in the cache.
- Decide if `diagram` should stay ASCII or accept a drawn format.
