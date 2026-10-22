# vistan

Turns a Markdown document into something a human can actually read: a page in the
browser, or clean text in the terminal. Made for the moment when an agent finishes
working and you do not want to read the whole conversation.

A single Go binary. No runtime dependencies, no interpreter, no build step for the
person using it.

## Why

Chat output is a transcript. A plan, a summary or a decision has a shape, and the
shape is what makes it readable. vistan takes a document that follows one of seven
shapes, checks it is complete, and renders it.

## Install

```sh
go install github.com/sthbryan/vistan@latest   # into $GOBIN
```

Or from a checkout:

```sh
go build -o vistan .
```

`brew install sthbryan/tap/vistan` works once a release exists; the formula expects
`vistan_x.y.z_darwin_arm64.tar.gz` and `..._darwin_amd64.tar.gz`.

## Commands

```sh
vistan <file.md>                 # print in the terminal
vistan serve <file.md>           # build the page, start the server, open it
vistan map [path] [--depth N]    # scan a directory and serve its map
vistan index                     # list every generated page and serve it
vistan stop                      # stop the server
vistan new <template>            # print a template to stdout
vistan templates                 # list templates
vistan serve --template map x.md # override the declared template
```

`serve` starts a detached HTTP server on `127.0.0.1:8765` and prints the URL. It
survives the command that launched it, because it is re-executed in its own session.
Later calls reuse it, so pages accumulate in the same origin. `VISTAN_PORT` changes
the port, `VISTAN_NO_OPEN=1` skips the browser, `VISTAN_CACHE` moves the generated
pages and the pid file.

Terminal exit codes: `0` complete, `1` a section is missing or empty, `2` cannot read
the file. That is the hook an agent can check before showing you anything.

## Templates

| Template | Sections | Use it for |
|---|---|---|
| `plan` | Decisions, Rejected, Verification, Your move (plus optional Steps and Files) | agreeing on work before doing it |
| `summary` | What changed, Verification, Pending | reporting work just finished |
| `selection` | one section per option | asking you to pick, with a copyable answer |
| `map` | Overview, Tree, Entry points, Largest files | orienting someone in a codebase |
| `diagram` | Context, Diagram, Reading | showing a flow or a layout |
| `adr` | Status, Context, Decision, Consequences (plus Options considered and Links) | recording why a design choice was made |
| `changelog` | one section per release, `### Added` / Changed / Fixed / Removed inside | telling users what changed in a release |

Each template is a real `.md` file in `templates/`, embedded into the binary, and
starts with a marker:

```markdown
<!-- vistan: map -->
```

That marker is how `vistan serve examples/map.md` knows which shape to draw; there is
no flag to remember. `--template` exists only to override it.

`plan`, `summary`, `map`, `diagram` and `adr` are validated: a missing or empty
section turns the footer into a warning and makes the terminal exit with `1`.
`selection` and `changelog` are free form, because their section names carry the
content.

`###` subheadings are allowed inside any section; `changelog` uses them for the
change groups, and `adr` gets its status rendered as a chip whose colour follows the
word (accepted, proposed, rejected, superseded, deprecated).

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

`vistan index` lists every generated page with its link and template name, writes
`index.html` into the cache with the same list, and serves it at the root URL. It is
how you get back to a page you generated an hour ago without remembering its name:

```
7 pages  http://127.0.0.1:8765/
  Where should generated pages live? selection http://127.0.0.1:8765/selection.html
  Changelog                          changelog http://127.0.0.1:8765/changelog.html
```

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
  drawn by the binary, not an image and not a dependency.
- **Overview.** File count, total size, lines counted and a bar chart of the top languages by lines.
- **Tree.** Directories first with their file count and size, `--depth` levels deep
  (default 2), a maximum of 13 entries per directory and `N more` for the rest.
- **Entry points.** `package.json` with its scripts, `go.mod` module, `Cargo.toml` and
  `pyproject.toml` names, `Makefile` targets, and the presence of `Dockerfile`,
  `compose.yaml` or `flake.nix`.
- **Largest files.** The five files with the most lines, falling back to size.

Lines are only counted for known text extensions under 2MB, so a repository full of
binaries does not stall the scan. A 150-file project takes about 60ms.

## What the renderer understands

A deliberate subset, not full Markdown:

- One `# ` title, `## ` sections, paragraphs, and bullet lists indented two spaces per level.
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

The one exception is the plan's **Your move**: it is a bordered panel tagged
`waiting on you`, because that is the part the reader has to act on. The summary strip
at the top (`6/8 checks`, `7 files`, `1 risk`) is built from the document itself, so it
is never out of date.

## Limits

- Pages are static. The selection page cannot notify the agent; the answer travels by
  clipboard.
- The server is a plain directory server with no auth. It binds to loopback only.
- The page is generated once; it does not reload when the Markdown changes.
- Selection mode turns every `##` heading into an option, so a stray section becomes
  an extra choice.

## Development

```sh
go test ./...     # parser, renderers, escaping, templates
go vet ./...
gofmt -l .
```

`vistan serve-child` is internal: it is the foreground process that `serve` spawns,
and it only serves the cache directory it is given.
