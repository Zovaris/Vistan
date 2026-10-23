# vistan

Markdown in, a page a human reads out.

A single Go binary that turns a Markdown document with a known shape into a page you
open once and understand: plans, reviews, ADRs, specs, changelogs, graphs, a map of a
project, and a history of everything you generated.

## Install

```sh
go install github.com/zovaris/vistan/cmd/vistan@latest
```

That builds the latest tagged release and drops `vistan` in `$GOBIN` — `~/go/bin`
unless you set it, so keep that on your `PATH`. No clone, no build step.

Or take an archive for macOS or Linux, on amd64 or arm64, from
[Releases](https://github.com/zovaris/vistan/releases), unpack it and move `vistan` into
your `PATH`. Every release publishes a `checksums.txt` next to the archives.

From a checkout, for working on vistan itself:

```sh
git clone https://github.com/zovaris/vistan.git
cd vistan
make install     # go install ./cmd/vistan
```

`vistan --version` prints the tag the binary was built from, or `dev` from a working
tree.

## Use

```sh
vistan new plan > plan.md     # start from a template
vistan serve plan.md          # build, serve on 127.0.0.1:8765 and open it
vistan serve --watch plan.md  # the same, rebuilt on every save until Ctrl-C
vistan plan.md                # the same document in the terminal
```

`serve` prints the URL and leaves a detached server behind, so pages accumulate at the
same origin. `vistan stop` ends it. `serve --watch` stays in the foreground instead,
rebuilds the page on every save, and the open tab reloads itself; Ctrl-C stops the
watcher, and the server keeps serving the last version. `vistan --help` prints the usage; it works after any
command (`vistan serve --help`), and `vistan brief` prints it after the brief. A flag
that takes a value reads either form: `--template map` or `--template=map`.

```sh
vistan map .                  # scan a directory into a map page
vistan docs docs/             # index every ADR and spec by status
vistan history                # the saved pages, grouped by project
vistan history --json         # the same list as JSON, for an agent
vistan open treemap           # reopen a page by name or prefix
vistan answer treemap         # the choice the human sent from the page
vistan rerender treemap       # rebuild a page from its stored source
vistan rm treemap             # delete saved pages
vistan clean                  # stop the server and delete every saved page
vistan uninstall              # clean, then delete the binary
vistan version                # the version of this binary
vistan brief                  # instructions for the agent that writes the pages
```

`clean` stops the server and removes the state directory, then says `nothing to
remove` if there was nothing left. It refuses a path that holds more than vistan's own
pages — a filesystem root, the home directory, the working directory — so a
`VISTAN_CACHE` pointed at the wrong place is an error instead of a wipe. `uninstall`
runs `clean` first and only then deletes the binary it is running from.

## Use it from an agent

vistan is a plain CLI, so any agent that can run shell commands can use it: no plugin,
no MCP server, no daemon to register. What it cannot guess is that vistan exists and
what shape the pages have. The brief is inside the binary for exactly that:

```sh
vistan brief >> AGENTS.md      # or CLAUDE.md, .cursor/rules, whatever the harness reads
```

That appends the commands, the template table, the markup that becomes a component, the
exit codes and the "do not" list. Do it once in the repository where the agent runs, not
in this one. The short version, if you would rather write it by hand:

```markdown
## Showing work to the human

For anything longer than a two-line answer, use `vistan`. Write `page.md` from a
template (`vistan new plan > page.md`), fill it, run `vistan serve page.md`, and give
the human the printed `http://127.0.0.1:8765/...` URL. Run `vistan brief` for the
sections and the markup. Do not paste the page or a raw diff into the chat.
```

## The choice comes back

A `selection` or a `review` page ends in one button. The human picks, presses **Send**,
and the page posts the choice to the same local server, which writes it next to the
page. The agent runs `vistan answer <name>` and gets `C: An index in ~/.vistan/` on
stdout, or `no answer yet` and exit `1` if nobody chose. Nothing is pasted, nothing is
copied through the clipboard, and nothing waits: the command answers immediately, and
the agent reads the choice on its next turn, when the human comes back to the chat.

The endpoint is `POST /_answer/<page>` on the loopback server. It refuses a request
whose `Origin` is not the page's own, so another site cannot answer on the human's
behalf, and it caps the body at 4 KB. The server binds `127.0.0.1` only, so over SSH
you have to forward the port before the button can reach it. Rendering a page again
deletes its old answer, so the agent never reads the choice from the previous run.

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

`plan`, `summary`, `selection`, `guide`, `review`, `map`, `diagram`, `graph`, `adr`,
`spec`, `changelog`. `vistan templates` lists them; `vistan new <name>` prints one.

## Where the pages live

The user state directory, not the project: `~/.local/state/vistan` on Linux,
`~/Library/Application Support/vistan` on macOS, `%AppData%\vistan` on Windows. At most
50 pages per project and 30 days; `VISTAN_KEEP` and `VISTAN_KEEP_DAYS` change that, so a
busy repository cannot evict the pages of another one.

Each page keeps its source next to it (`plan.html` and `plan.md`), so it can be
re-rendered with the current templates and read as plain text:

```sh
vistan rerender plan          # picks the template from the stored marker
vistan rerender --all         # rebuild every page that has a source
```

## More

- [INSTRUCTIONS.md](INSTRUCTIONS.md): every command, template, the renderer subset and
  the repository layout.
- `vistan brief`: the same rules, written for the agent that generates the pages. This
  is the text to paste into the harness that will use vistan; see above.

## Development

```sh
make check      # gofmt, go vet and the tests
make lint       # golangci-lint
```

CI runs `golangci-lint` and, on Go 1.22 and the current release, the formatter, `vet`
and the tests. `lefthook install` links the same four checks into `.git/hooks` once
(`brew install lefthook` if you do not have it): a commit formats the Go files it
touches and stages the result, and a push runs everything.

## Releases

The tag is the version: nothing in the repository stores it. `make version VERSION=0.1.0`
checks the tree is clean on `main`, creates the annotated tag and pushes it;
`make version BUMP=minor` raises the last tag instead, `make next-version` only prints
what that would do, and `DRY=1` or `NO_PUSH=1` stop before touching the remote. Pushing
the tag runs `.github/workflows/release.yml`, which repeats the checks, confirms the
binary reports that tag, and lets goreleaser build and publish the archives.
