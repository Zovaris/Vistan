<!-- vistan: plan -->

# vistan: a page a human reads

## Decisions

- Documents are Markdown with fixed sections; the tool never invents content.
- One shell script plus awk, no dependencies, no build step.
- `serve` runs a local Python http.server on 127.0.0.1 and leaves it running.
- Templates live as `.md` files, so the agent only has to copy one.

## Rejected

- A packaged binary: builds, signing and distribution for a 300 line script.
- JSON documents: Markdown survives a broken parser and humans can read it.
- A subagent writing the summary: it costs a call and can invent the state.
- Rendering with a JS framework: the page is text, not an app.

## Verification

- `./test.sh` covers rendering, exit codes, escaping and the server lifecycle.
- `curl` against the served page returns 200 with the expected sections.
- The selection page was clicked in a browser: the answer field fills and Copy works.

## Your move

- Decide whether the page should keep the paper look at night or go dark.
- Decide if the cache stays in TMPDIR or moves to a folder you can revisit.
