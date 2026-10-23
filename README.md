# Vistan

<p align="center">
  <img src="./assets/brand/vistan-icon.png" alt="Vistan" width="112" height="112" />
</p>

<p align="center">
  <strong>Markdown in, a page a human reads out.</strong><br />
  A single Go binary that renders plans, reviews, ADRs, specs, changelogs and maps as pages served from localhost.
</p>

Vistan takes a Markdown document with a known shape, checks it against one of eleven
templates and renders it as a self-contained page on `127.0.0.1`. A plan, a review or a
decision can end in a button, so the human's choice comes back to the agent instead of
being pasted into a chat. No runtime, no dependencies, no build step for the person
reading the page.

## Requirements

- macOS or Linux, on amd64 or arm64
- Go 1.22 or later, when building from source
- Nothing at runtime: the binary embeds its templates, its CSS and its brief

## Install

One command, with no Go and no Homebrew in the way:

```bash
curl -fsSL https://raw.githubusercontent.com/zovaris/vistan/main/install.sh | sh
```

It reads the latest tag, downloads the archive for your platform, checks it against the
release's `checksums.txt` and puts `vistan` in `/usr/local/bin`, or in `~/.local/bin` when
that one is not writable. `| sh -s -- v0.1.0` installs a specific release (the `v` is
optional), and `VISTAN_BIN_DIR` chooses another directory.

On macOS, with Homebrew:

```bash
brew install sthbryan/tap/vistan
```

With Go:

```bash
go install github.com/zovaris/vistan/cmd/vistan@latest
```

That puts `vistan` in `$GOBIN` (`~/go/bin` unless you set it), so keep that on your
`PATH`. Or take the archive for your platform from
[Releases](https://github.com/zovaris/vistan/releases), unpack it and move `vistan` into
your `PATH`; every release publishes a `checksums.txt` next to the archives.

From a checkout, for working on Vistan itself:

```bash
git clone https://github.com/zovaris/vistan.git
cd vistan
make install
```

`vistan --version` prints the tag the binary was built from, or `dev` from a working tree.

## Use

Write the page, then let Vistan render it:

```bash
vistan new plan > plan.md     # start from a template
vistan serve plan.md          # build it, serve it on 127.0.0.1:8765 and open it
vistan serve --watch plan.md  # the same, rebuilt on every save until Ctrl-C
vistan plan.md                # the same document in the terminal
```

`serve` prints the URL and leaves a detached server behind, so pages accumulate at the
same origin; `vistan stop` ends it. `serve --watch` stays in the foreground instead,
rebuilds the page on every save and the open tab reloads itself. `vistan --help` prints
the usage and works after any command (`vistan serve --help`), and a flag that takes a
value reads either form: `--template map` or `--template=map`.

The rest of the commands:

```bash
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
vistan update                 # install the latest release over the binary
vistan version                # the version of this binary
vistan brief                  # the rules for the agent that writes the pages
```

`update` compares the binary with the latest release and, when there is a newer one,
downloads its archive, checks it against `checksums.txt` and replaces the binary in place.
A Homebrew install is told to `brew upgrade vistan` instead of being overwritten under it.

`clean` stops the server and removes the state directory, then says `nothing to remove`
if there was nothing left. It refuses a path that holds more than Vistan's own pages — a
filesystem root, the home directory, the working directory — so a `VISTAN_CACHE` pointed
at the wrong place is an error instead of a wipe. `uninstall` runs `clean` first and only
then deletes the binary it is running from.

## Templates

`plan`, `summary`, `selection`, `guide`, `review`, `map`, `diagram`, `graph`, `adr`,
`spec` and `changelog`. `vistan templates` lists them and `vistan new <name>` prints one.
The template is declared on the first line, `<!-- vistan: plan -->`, and the `##` section
names are the contract, not decoration.

## Input

Markdown, not JSON or TOML. One marker, one `#` title, and `##` sections with the names
the template expects.

````markdown
<!-- vistan: plan -->

# Migrate the cache

Two lines of intro.

## Decisions

- keep the Markdown

## Verification

- [x] go test ./...
````

`vistan plan.md` exits `1` when a required section is missing; `serve` renders anyway and
warns in the footer. [examples/input.md](examples/input.md) is the long version, rendered
by Vistan itself.

## Where the pages live

The user state directory, not the project: `~/.local/state/vistan` on Linux and
`~/Library/Application Support/vistan` on macOS. At most 50 pages per project and 30 days;
`VISTAN_KEEP` and `VISTAN_KEEP_DAYS` change that, so a busy repository cannot evict the
pages of another one.

Each page keeps its source next to it (`plan.html` and `plan.md`), so it can be
re-rendered with the current templates and read as plain text:

```bash
vistan rerender plan          # picks the template from the stored marker
vistan rerender --all         # rebuild every page that has a source
```

## The choice comes back

A `selection` or a `review` page ends in one button. The human picks, presses **Send**, and
the page posts the choice to the same local server, which writes it next to the page. The
agent runs `vistan answer <name>` and gets `C: An index in ~/.vistan/` on stdout, or
`no answer yet` and exit `1` if nobody chose. Nothing is pasted, nothing goes through the
clipboard, and nothing waits: the command answers immediately and the agent reads the
choice on its next turn.

The endpoint is `POST /_answer/<page>` on the loopback server. It refuses a request whose
`Origin` is not the page's own, so another site cannot answer on the human's behalf, and
caps the body at 4 KB. The server binds `127.0.0.1` only, so over SSH you have to forward
the port before the button can reach it. Rendering a page again deletes its old answer, so
an agent never reads the choice from the previous run.

## Use it from an agent

Vistan is a plain CLI, so any agent that can run shell commands can use it: no plugin, no
MCP server, no daemon to register. What it cannot guess is that Vistan exists and what
shape the pages have. The brief is inside the binary for exactly that:

```bash
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

## Build and test

```bash
make build      # build ./vistan
make check      # gofmt, go vet and the tests
make lint       # golangci-lint
make help       # every target
```

CI runs `golangci-lint` and, on Go 1.22 and the current release, the formatter, `vet` and
the tests. `lefthook install` links the same four checks into `.git/hooks` once — a commit
formats the Go files it touches and stages the result, and a push runs everything.
[INSTRUCTIONS.md](INSTRUCTIONS.md) has every command, the renderer subset and the
repository layout.

## Releases

The tag is the version: nothing in the repository stores it. `make version VERSION=0.1.0`
(or `VERSION=v0.1.0`, the tag is written with the `v` either way) checks the tree is clean
on `main`, creates the annotated tag and pushes it;
`make version BUMP=minor` raises the last tag instead, `make next-version` only prints what
that would do, and `DRY=1` or `NO_PUSH=1` stop before touching the remote. Pushing the tag
runs `.github/workflows/release.yml`, which repeats the checks, confirms the binary reports
that tag, and lets goreleaser build and publish the archives.

## License

Vistan is licensed under the GNU General Public License, version 3 or any later version.
See [`LICENSE`](LICENSE).
