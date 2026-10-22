# vistan

Markdown in, a page a human reads out.

A single Go binary that turns a Markdown document with a known shape into a page you
open once and understand: plans, reviews, ADRs, specs, changelogs, graphs, a map of a
project, and a history of everything you generated.

## Install

From a checkout, which is the path that works today:

```sh
git clone https://github.com/sthbryan/vistan.git
cd vistan
go install ./cmd/vistan     # into $GOBIN
```

Or build the binary in place:

```sh
go build -o vistan ./cmd/vistan
```

The repository is private for now, so the module path only resolves for someone with
access and Go's git credentials:

```sh
GOPRIVATE=github.com/sthbryan go install github.com/sthbryan/vistan/cmd/vistan@latest
```

Once it is public, that becomes plain `go install github.com/sthbryan/vistan/cmd/vistan@latest`.
`brew install sthbryan/tap/vistan` works once a release exists.

## Use

```sh
vistan new plan > plan.md     # start from a template
vistan serve plan.md          # build, serve on 127.0.0.1:8765 and open it
vistan plan.md                # the same document in the terminal
```

`serve` prints the URL and leaves a detached server behind, so pages accumulate at the
same origin. `vistan stop` ends it.

```sh
vistan map .                  # scan a directory into a map page
vistan docs docs/             # index every ADR and spec by status
vistan history                # the saved pages, grouped by project
vistan history --json         # the same list as JSON, for an agent
vistan open treemap           # reopen a page by name or prefix
vistan rm treemap             # delete saved pages
vistan brief                  # instructions for the agent that writes the pages
```

## Input

Markdown, not JSON or TOML. The shape is the contract: a marker that picks the
template, one `#` title, and `##` sections with the names the template expects.

````markdown
<!-- vistan: plan -->

# Migrate the cache

Two lines of intro.

## Decisions

- keep the Markdown

## Verification

- [x] go test ./...
````

`vistan plan.md` exits `1` when a required section is missing; `serve` renders anyway
and warns in the footer. [examples/input.md](examples/input.md) is the long version,
rendered by vistan itself.

## Templates

`plan`, `summary`, `selection`, `review`, `map`, `diagram`, `graph`, `adr`, `spec`,
`changelog`. `vistan templates` lists them; `vistan new <name>` prints one.

## Where the pages live

The user state directory, not the project: `~/.local/state/vistan` on Linux,
`~/Library/Application Support/vistan` on macOS, `%AppData%\vistan` on Windows. At most
50 pages and 30 days; `VISTAN_KEEP` and `VISTAN_KEEP_DAYS` change that.

## More

- [INSTRUCTIONS.md](INSTRUCTIONS.md): every command, template, the renderer subset and
  the repository layout.
- `vistan brief`: the same rules, written for the agent that generates the pages.

## Development

```sh
go test ./...
go vet ./...
gofmt -l .
```
