<!-- vistan: map -->

# vistan

A single shell script that renders Markdown as a page for a human, plus a local server.

## Entry points

- `vistan`: the whole program. `main` parses the command, then calls `render_terminal` or `render_page`.

## Modules

- `templates/`: the five document shapes an agent copies.
  - `plan.md`, `summary.md`, `selection.md`, `map.md`, `diagram.md`
- `examples/`: one filled document per template, also used by the tests.
- `test.sh`: thirteen checks over the CLI, the HTML and the server.

## Data flow

- `.md` -> awk parse -> terminal text
- `.md` -> awk parse -> HTML in `$TMPDIR/vistan` -> python http.server -> browser

## Gotchas

- The HTML emitter and the terminal emitter share one parser; touching one usually touches both.
- Selection mode turns every `##` heading into an option, so a stray section becomes an extra choice.
- `start_server` keeps the PID in the cache folder; deleting that folder orphans the server.
