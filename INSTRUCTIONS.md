# vistan

Turns a Markdown document into something a human can actually read: a page in the
browser, or clean text in the terminal. Made for the moment when an agent finishes
working and you do not want to read the whole conversation.

A single Go binary. No runtime dependencies, no interpreter, no build step for the
person using it.

## Why

Chat output is a transcript. A plan, a summary or a decision has a shape, and the
shape is what makes it readable. vistan takes a document that follows one of ten
shapes, checks it is complete, and renders it.

`vistan brief` prints the instructions for the agent that writes those pages, so an
orchestrator can hand all the page work to a cheap sub-agent.

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

The repository is private for now, so the module path only resolves with access and
Go's git credentials:

```sh
GOPRIVATE=github.com/sthbryan go install github.com/sthbryan/vistan/cmd/vistan@latest
```

`brew install sthbryan/tap/vistan` works once a release exists; the formula expects
`vistan_x.y.z_darwin_arm64.tar.gz` and `..._darwin_amd64.tar.gz`.

## Commands

```sh
vistan <file.md>                 # print in the terminal
vistan serve <file.md>           # build the page, start the server, open it
vistan map [path] [--depth N]    # scan a directory and serve its map
vistan docs [path]               # index every ADR and spec under a directory
vistan history [--json]          # the saved pages, grouped by project, or as JSON
vistan index                     # same as history
vistan open <name>               # reopen a saved page by name or prefix
vistan answer <name>             # the choice the human sent from the page
vistan rerender <name>           # rebuild a saved page from its stored source
vistan rerender --all            # rebuild every saved page that has a source
vistan rm <name>                 # delete saved pages
vistan brief                     # the instructions for the agent that writes pages
vistan stop                      # stop the server
vistan clean                     # stop the server and delete the saved pages
vistan uninstall                 # clean, then delete the binary
vistan new <template>            # print a template to stdout
vistan templates                 # list templates
vistan serve --template map x.md # override the declared template
vistan <command> --help         # the usage, from any command
```

`-h`, `--help` and `help` print the usage on stdout and exit `0`, wherever they appear
in the command line, so `vistan serve --help` is the same text as `vistan --help`.
`vistan brief` prints the brief and then the same usage, so an agent that reads only
that one command still sees the env vars and the command list.

`answer` prints the choice a human sent from a `selection` or a `review` page, one
line on stdout, and exits `0`. With no answer yet it prints `no answer yet for <name>`
on stderr and exits `1`, which is what a polling agent checks; a page that does not
exist is `2`. The record is `<state>/<name>.answer.json`, next to the page and its
source, and it is removed with the page.

Pages do not ship a copy button. Both panels post to `POST /_answer/<page>` on the
loopback server, which writes the record atomically and answers `204`. The handler
refuses a foreign `Origin` (`403`) so another site cannot answer for the human, caps
the body at 4 KB, and rejects an empty answer (`400`). The page derives its own name
from `location.pathname`, so a renamed page keeps working.

`clean` stops the server, then removes the state directory: the pages, their stored
sources, the index and the pid file. Running it again prints `nothing to remove` and
exits `0`. It refuses a path that holds more than vistan's pages — a filesystem root,
the home directory, the working directory — because `VISTAN_CACHE` set to the wrong
directory would otherwise turn a cleanup into a wipe. With `VISTAN_CACHE` unset it also
deletes the old `$TMPDIR/vistan` directory. `uninstall` runs `clean` and then deletes
the running binary; if the file cannot be unlinked (Windows locks a running `.exe`), it
prints the path to remove by hand and exits `2`.

`serve` starts a detached HTTP server on `127.0.0.1:8765` and prints the URL. It
survives the command that launched it, because it is re-executed in its own session.
Later calls reuse it, so pages accumulate in the same origin. `VISTAN_PORT` changes
the port, `VISTAN_NO_OPEN=1` skips the browser, `VISTAN_CACHE` moves the generated
pages and the pid file.

Pages are not temporary. They live in the user state directory and survive a reboot
and a temp cleanup:

| System | Path |
|---|---|
| Linux | `$XDG_STATE_HOME/vistan` or `~/.local/state/vistan` |
| macOS | `~/Library/Application Support/vistan` |
| Windows | `%AppData%\vistan` |

Every render writes two files there: the page and its source, `plan.html` and
`plan.md`. The source is the exact document vistan received, so a page can be
re-rendered later with the current templates (`vistan serve <dir>/plan.md`) or read as
plain text without parsing HTML. `map` is the exception: it has no source, so only the
page is written.

The oldest pages are deleted both by count and by age: at most `VISTAN_KEEP` pages
(default 50, `0` disables the count) and nothing older than `VISTAN_KEEP_DAYS` (default
30, `0` disables the age limit). A page and its source always leave together. Retention
runs after a page is written, so the directory never grows without bound and `history`
never links to a file that is gone. The count is the real disk bound; the age ceiling is what keeps
a page from a quiet month from lingering forever.

Terminal exit codes: `0` complete, `1` a section is missing or empty, `2` cannot read
the file. That is the hook an agent can check before showing you anything.

## Templates

| Template | Sections | Use it for |
|---|---|---|
| `plan` | Decisions, Rejected, Verification, Your move (plus optional Steps and Files) | agreeing on work before doing it |
| `summary` | What changed, Verification, Pending | reporting work just finished |
| `selection` | one section per option | asking you to pick, with a copyable answer |
| `guide` | no required sections; numbered steps work for a walkthrough | explaining how something works |
| `map` | Overview, Git, Tree, Entry points, Largest files | orienting someone in a codebase |
| `diagram` | Context, Diagram, Reading | showing a flow or a layout |
| `graph` | Context, Graph, Reading | boxes and arrows between named things, drawn as SVG |
| `adr` | Status, Context, Decision, Consequences (plus Options considered and Links) | recording why a design choice was made |
| `spec` | Status, Problem, Proposal, Decisions (plus Open questions and Links) | proposing a change before it is built |
| `changelog` | one section per release, `### Added` / Changed / Fixed / Removed inside | telling users what changed in a release |
| `review` | one section per finding, named `F1 · Blocker · path:42`, plus Tests and Notes | showing several reviewers' findings, with a decision per finding |

Each template is a real `.md` file in `internal/vistan/templates/`, embedded into the binary, and
starts with a marker:

```markdown
<!-- vistan: map -->
```

That marker is how `vistan serve examples/map.md` knows which shape to draw; there is
no flag to remember. `--template` exists only to override it.

`plan`, `summary`, `map`, `diagram`, `graph`, `adr` and `spec` are validated: a
missing or empty section turns the footer into a warning and makes the terminal exit
with `1`. `selection` and `changelog` are free form, because their section names carry
the content.

`###` subheadings are allowed inside any section; `changelog` uses them for the
change groups, and `adr` gets its status rendered as a chip whose colour follows the
word (accepted, proposed, rejected, superseded, deprecated).

## The review page

A `review` document is one section per finding, and the section name carries the
finding:

```markdown
## F1 · Blocker · `render.go:212`
```

That renders as a card with the id, a severity chip (blocker, critical, major,
important, minor, nit, question, pre-existing, or any other word), the location in
monospace, and a checkbox. The body holds the claim and the fix, with code if it helps.

At the bottom the page adds a decision panel: tick the findings to apply and copy
`Apply F1, F3`, or copy `Reject all findings`. That line is the answer the agent reads
back; nothing is sent anywhere by itself. The summary strip counts the findings by
severity and the checkboxes in `## Tests`, so the reader sees the state before reading
a single finding.

The section names that do not look like a finding (`Tests`, `Notes`, `Summary`) render
as normal sections.

## Delegating the page work

`vistan brief` prints [BRIEF.md](internal/vistan/BRIEF.md): what the tool does, which template to pick,
the markup that becomes a component, the review workflow and the exit codes, followed
by the usage. Give that text to the sub-agent that writes the pages, and keep the
orchestrator for deciding what to review and what to apply. The brief is embedded in
the binary, so it always matches the version that renders the page.

## The agent workflow

1. `vistan new plan > plan.md`
2. Fill the sections. Do not add sections the template does not have.
3. `vistan serve plan.md`
4. The human reads the page instead of the transcript.

For a decision:

1. `vistan new selection > choice.md`
2. Write the question as the title and one section per option.
3. `vistan serve choice.md`
4. The human picks a radio, copies `B: <option>`, pastes it back.

## The index

`vistan history` lists every saved page with its time, template, project and link,
writes `index.html` with the same list, and serves it at the root URL. It is how you
get back to a page you generated an hour ago without remembering its name:

```
6 pages · 2 projects  http://127.0.0.1:8765/
vistan  4 pages · last today 14:32
  14:32  review    Review: the treemap and the index   http://.../review.html
  14:32  plan      vistan: a page a human reads        http://.../plan.html
fizza   2 pages · last 20 Oct
  11:04  adr       ADR: serve pages over HTTP           http://.../adr.html
```

Every render also rewrites `index.html`, so the `← History` link at the top of each
page always resolves, even before you run `vistan history` for the first time.

Above the groups, a strip of 30 squares shows how many pages each of the last 30 days
produced; darker is more, and the tooltip carries the date and the count. It is computed
from the file timestamps, so nothing extra is stored for it.

The page groups entries by project, newest activity first, with a count and the last
time each project generated something. The project is the nearest directory with a
`.git`, not the directory the command ran in, so a page built from `examples/` still
belongs to the repository.

`vistan history --json` prints the same list for an agent: `dir`, and for each page
`name`, `title`, `template`, `project`, `source` (empty when there is none), `modified`
(RFC 3339) and `bytes`. It writes
`index.html` but starts no server and opens no browser, so a script can call it freely.

`vistan open treemap` reopens the one page whose name contains that, and refuses when
the name matches several. `vistan rerender treemap` reads the stored `.md`, picks the
template from its marker, and rewrites the page with the current CSS and templates. It
keeps the original source label and the file timestamp, so the history order and the
age limit do not shift; `map` pages have no source and cannot be re-rendered. `vistan
rerender --all` does the same for every saved page, skips the ones without a source and
prints how many it rebuilt, so changing a template is one command; it exits `2` when a
page fails, after rebuilding the rest. `vistan
rm` deletes pages and rewrites `index.html`, so the list never links to a file that is
gone.

## The map command

`vistan map <path>` needs no input document: it scans the directory and builds the
`map` shape itself.

- **File list.** Inside a git repository it runs `git ls-files --cached --others
  --exclude-standard`, so your own `.gitignore` decides what counts. Outside a repo it
  walks the tree and skips `node_modules`, `.git`, `vendor`, `dist`, `build`, `target`,
  `.next`, `out`, `__pycache__`, `.venv`, `venv`, `.idea`, `.cache`, `coverage`.
- **Treemap.** Above the sections, an inline SVG shows where the bytes are: area is
  proportional to size, the squarified layout keeps boxes close to square, colour marks
  the largest top-level entries and a child box inherits its parent's colour. It is SVG
  drawn by the binary, not an image and not a dependency. Tiles too small for a label
  carry their name in a `<title>` and show it in a single line under the map on hover,
  so the drawing never fills up with text. Below 560px of viewport the drawing is
  replaced by a list of the top-level entries with a proportional bar, because the
  labels inside the SVG shrink with it.
- **Overview.** File count, total size, lines counted and a bar chart of the top languages by lines.
- **Git.** The current branch, the last commit with its short hash and age, and the files that
  are still uncommitted, each with its `M`/`A`/`D`/`?` letter. The section is left out
  entirely outside a repository, or when `git` is not on the path.
- **Tree.** Directories first with their file count and size, `--depth` levels deep
  (default 2), a maximum of 13 entries per directory and `N more` for the rest.
- **Entry points.** `package.json` with its scripts, `go.mod` module, `Cargo.toml` and
  `pyproject.toml` names, `Makefile` targets, and the presence of `Dockerfile`,
  `compose.yaml` or `flake.nix`.
- **Largest files.** The five files with the most lines, falling back to size.

Lines are only counted for known text extensions under 2MB, so a repository full of
binaries does not stall the scan. A 150-file project takes about 60ms.

## The graph page

A `graph` document draws its `## Graph` section as SVG, from an edge list:

```
web --> api
api --> db
api --> queue : async
api [API server]
```

Each line is an edge (`-->`, `->`, `=>` or `→`); the text after ` : ` is the edge
label; a line with only a name and `[label]` names a node. Nodes are placed in layers
by following the arrows, so the drawing comes from the edges and not from coordinates
nobody wants to maintain. An edge that points back up is drawn dashed around the right
margin, because a loop is not a hierarchy. Arrowheads are an SVG marker and the
colours are the page's own variables, so dark mode works. The page is 52rem wide and
nothing is fetched: the SVG is inline, like the treemap.

## The decisions index

`vistan docs <path>` walks a directory, takes every `.md` that declares
`<!-- vistan: adr -->` or `<!-- vistan: spec -->` — or that has both a `Status` and a
`Decision` section — renders each into its own page, and writes `decisions.html`:
records grouped by status, newest first inside each group, with kind, date and source
path.

The status is the first word of the `## Status` section; the date is whatever follows an
em dash, `|` or `·`, and falls back to the file's modification time. Groups run
accepted, approved, proposed, draft, rejected, superseded, deprecated, then anything
else. `templates` and the usual build directories are skipped, so the template that
starts every document is not indexed as a record.

Exit `1` when nothing matches, `2` when the directory cannot be read.

## What the renderer understands

A deliberate subset, not full Markdown:

- One `# ` title, `## ` sections, paragraphs, bullet lists and numbered lists (`1.`),
  indented two spaces per level.
- `###` subheadings, rendered as small caps labels.
- Task lists: `- [x] done` and `- [ ] todo` become checkboxes; a section header shows
  `done/total` and the page gets a summary strip (`6/8 checks · 7 files · 1 risk`).
- Callouts: a blockquote that starts with `> [!WARN] Title` renders as a labelled panel.
  `note`, `tip`, `warn`, `risk`, `decision` and `question` have their own colour.
- A fenced block tagged `changes` renders a file list instead of code:

  ```
  + new/file.go
  ~ edited/file.go  what changed in it
  - gone/file.go
  > old.go -> new.go
  ```

  `+` add, `~` modify, `-` delete, `>` move; the note after two spaces is optional.
- Fenced code blocks, shown in a bordered monospace box with horizontal scroll.
- Inline `` `code` ``, `**bold**`, `[links](https://example.com)` and bold that contains
  code. Links are only rendered for `http`, `https`, `mailto`, `#` and relative paths.
- Images: `![caption](path.png)` is inlined into the page as a data URI, so the HTML stays
  self-contained and works without the original file next to it. Anything over 3MB is left
  as a plain reference. A `selection` option can carry figures, code and prose, not just
  bullets.

Anything else stays literal text, which is why a broken document still renders.

## How it looks

A paper document, not a dashboard: warm off-white, near-black ink, serif body at a
38rem measure, one hairline rule per section, italic section headings with a muted
count. Dark mode follows `prefers-color-scheme`. No web fonts, no shadows, no cards.
The tab carries an inline SVG favicon and every page opens with a small `← History`
link, so the browser tab and the page itself both say where you are.

The one exception is the plan's **Your move**: it is a bordered panel tagged
`waiting on you`, because that is the part the reader has to act on. The summary strip
at the top (`6/8 checks`, `7 files`, `1 risk`) is built from the document itself, so it
is never out of date.

## Limits

- Pages are static. A selection or review page posts the choice to the local server,
  but nothing pushes it to the agent: it reads it with `vistan answer`, and only when
  it asks.
- The server is a plain directory server with no auth. It binds to loopback only.
- The page is generated once; it does not reload when the Markdown changes.
- Selection mode turns every `##` heading into an option, so a stray section becomes
  an extra choice.

## Layout

```
cmd/vistan/        the binary: main calls vistan.Run
internal/vistan/   the program
  app.go           commands, flags, state directory, retention
  doc.go           parser and template list
  page.go          the HTML page, findings, selection, project name
  markup.go        inline markup, body, callouts, changes, images
  terminal.go      the terminal renderer and its colours
  styles.go        the page CSS and the favicon
  graph.go         the edge list to SVG
  docs.go          the ADR and spec index
  index.go         the saved pages, open and rm
  scan.go          project scan and treemap
  server.go        the local server and the pid file
  templates/       the embedded templates
  BRIEF.md         the embedded brief
```

The layout is for humans: `go install` still points at the module, and the binary is
one package. Splitting by layer would only add exported names and imports.

## Development

```sh
go test ./...     # parser, renderers, escaping, templates
go vet ./...
gofmt -l .
```

`vistan serve-child` is internal: it is the foreground process that `serve` spawns,
and it only serves the cache directory it is given.
