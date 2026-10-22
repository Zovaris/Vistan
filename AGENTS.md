# AGENTS.md

vistan turns a Markdown file into a page a human reads and decides on, served over
HTTP from a local port. It is a plain CLI: no daemon, no API, no MCP server. An agent
uses it by writing a file and running one command.

## In this repository

`go install ./cmd/vistan` (Go 1.22+) puts the binary on `PATH`. Then:

```sh
vistan brief                     # the rules: commands, templates, markup, exit codes
vistan new plan > page.md        # a template to fill
vistan serve page.md             # render, serve, print the URL
```

## In any other project

1. Write the page with its template on the first line: `<!-- vistan: plan -->`. The
   `##` section names are the contract, not decoration. `vistan brief` has the list.
2. `vistan serve page.md` renders it, keeps a detached server on `127.0.0.1:8765` and
   prints the URL.
3. Hand the human that URL. Do not paste the page into the chat and do not describe it
   instead of linking it.

`vistan page.md` without `serve` only prints to the terminal: no page, no server, no
storage. If the answer fits in two lines, say it in two lines. A page is for something
the human must read or decide.

## What the agent needs to know

- **Install.** Clone and `go install ./cmd/vistan`. The repo is private, so
  `go install github.com/sthbryan/vistan/cmd/vistan@latest` also needs
  `GOPRIVATE=github.com/sthbryan/*` and git credentials. The binary must be on the
  `PATH` of the process running the agent.
- **Where pages live.** The user state directory — `~/.local/state/vistan` on Linux,
  `~/Library/Application Support/vistan` on macOS, `%AppData%\vistan` on Windows.
  Pages survive closing the tab and rebooting; at most 50, nothing older than 30 days.
- **Naming.** A page is named after its file, not its project: two repos that both
  write `plan.md` overwrite each other. Name pages after the task.
- **Coming back.** `vistan history` lists saved pages and serves them, `vistan open
  <name>` reopens one, `vistan rerender <name>` rebuilds it from the stored source,
  `vistan rm <name>` deletes it.
- **Exit codes.** `0` complete, `1` a required section is missing or empty, `2` the
  page could not be written. `serve` renders anyway and warns in the footer.

## Drop-in block

For another repository's `AGENTS.md` or `CLAUDE.md`:

```markdown
## Showing work to the human

For anything longer than a two-line answer, use `vistan`. Write `page.md` from a
template (`vistan new plan > page.md`), fill it, run `vistan serve page.md`, and give
the human the printed `http://127.0.0.1:8765/...` URL. Run `vistan brief` for the
sections and the markup. Do not paste the page or a raw diff into the chat.
```
