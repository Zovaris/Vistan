# vistan

Turns a Markdown document into something a human can actually read: a page in the
browser, or clean text in the terminal. Made for the moment when an agent finishes
working and you do not want to read the whole conversation.

One file, `vistan`, about 300 lines of bash and awk. No dependencies beyond `python3`
for the server. No build step.

## Why

Chat output is a transcript. A plan, a summary or a decision has a shape, and the
shape is what makes it readable. vistan takes a document that follows one of five
shapes, checks it is complete, and renders it.

## Install

```sh
chmod +x vistan
ln -s "$PWD/vistan" ~/bin/vistan      # or add this folder to PATH
```

## Commands

```sh
vistan <file.md>                 # print in the terminal
vistan serve <file.md>           # build the page, start the server, open it
vistan stop                      # stop the server
vistan new <template>            # print a template to stdout
vistan templates                 # list templates
vistan serve --template map x.md # override the declared template
```

`serve` starts `python3 -m http.server` on `127.0.0.1:8765`, detached, and prints the
URL. It reuses the running server on later calls, so pages accumulate in the same
origin. `VISTAN_PORT` changes the port, `VISTAN_NO_OPEN=1` skips the browser.

Terminal exit codes: `0` complete, `1` a section is missing or empty, `2` cannot read
the file. That is the hook an agent can check before showing you anything.

## Templates

| Template | Sections | Use it for |
|---|---|---|
| `plan` | Decisions, Rejected, Verification, Your move | agreeing on work before doing it |
| `summary` | What changed, Verification, Pending | reporting work just finished |
| `selection` | one section per option | asking you to pick, with a copyable answer |
| `map` | Entry points, Modules, Data flow, Gotchas | orienting someone in a codebase |
| `diagram` | free form around one fenced diagram | showing a flow or a layout |

Each template is a real `.md` file in `templates/` and starts with a marker:

```markdown
<!-- vistan: map -->
```

That marker is how `vistan serve examples/map.md` knows which shape to draw; there is
no flag to remember. `--template` exists only to override it.

`plan`, `summary` and `map` are validated: a missing or empty section turns the footer
into a warning and makes the terminal exit with `1`. `selection` and `diagram` are free
form.

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

## How it looks

A paper document, not a dashboard: warm off-white, near-black ink, serif body at a
38rem measure, one hairline rule per section, italic section headings with a muted
count. Dark mode follows `prefers-color-scheme`. No web fonts, no shadows, no cards.

## Limits

- Pages are static. The selection page cannot notify the agent; the answer travels by
  clipboard.
- The server is a plain directory server with no auth. It binds to loopback only.
- The page is generated once; it does not reload when the Markdown changes.
- One parser serves both outputs; changing the HTML emitter usually means touching the
  terminal one too.

## Tests

```sh
./test.sh
```

Covers the CLI, the three validation outcomes, HTML escaping, the selection page and
the server lifecycle.
