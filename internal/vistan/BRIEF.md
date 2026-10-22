# vistan brief

You are the agent in charge of vistan for this task. You write the Markdown, run the
binary, and hand the human a URL. You do not paste the conversation into the chat.

## The rule

The human reads the page, not the transcript. One page per thing they must read or
decide. If the answer fits in two lines, say it in two lines — do not build a page.

## Commands

```sh
vistan new <template> > page.md     # start from the template
vistan serve page.md                # build, serve and open the page
vistan <page.md>                    # same document in the terminal
vistan map <path>                   # scan a directory into a map page
vistan docs [path]                  # index every ADR and spec into one page
vistan history                      # saved pages, grouped by project
vistan history --json               # the same list as JSON, no server
vistan open <name>                  # reopen a saved page by name or prefix
vistan rerender <name>              # rebuild a saved page from its stored source
vistan rerender --all               # rebuild every saved page that has a source
vistan rm <name>                    # delete saved pages
vistan templates                    # list template names
vistan brief                        # this file
vistan stop                         # stop the local server
```

`serve` prints the URL and keeps a detached server on `127.0.0.1:8765`, so the page
survives the command. Pass the URL to the human; do not describe the page instead.

Pages are kept between runs in the user state directory: at most 50, nothing older
than 30 days. A page from a month ago is gone even if the directory is almost empty.
Each page keeps its source Markdown next to it, so `vistan rerender plan` rebuilds it
with the current templates, and `vistan rerender --all` rebuilds every page at once and the source can be read as text. `vistan history` groups them by project with a count and
the last time each project wrote one, so a page from an hour ago is one command away.
Use `vistan history --json` when you need the list as data instead of prose.


## Pick the template

| Template | Use it when |
|---|---|
| `plan` | the human must agree before you start: decisions, rejected options, steps, files, verification, what you need from them |
| `summary` | work is finished and the human needs what changed, what you verified and what is pending |
| `selection` | one decision with 2–4 options; each option can carry an image, code and prose |
| `review` | several reviewers looked at a change: findings by severity, tests, and what to apply |
| `adr` | one decision that must stay recorded, with context, options and consequences |
| `changelog` | what changed per release, grouped by version |
| `diagram` | a flow or a layout that prose makes slower to understand |
| `graph` | named things and the arrows between them; the `## Graph` section is an edge list |
| `spec` | a change proposed before it is built: problem, proposal, decisions, open questions |
| `map` | orienting someone in a codebase; `vistan map` fills it by itself |

Declare the template with the first line of the document: `<!-- vistan: plan -->`.
`vistan new plan` already writes it.

## Markup that becomes a component

````markdown
## Section                 a section
- [x] done step            a checkbox, counted in the header and the summary strip
- [ ] pending step
> [!WARN] Title            a labelled callout: note, tip, warn, risk, decision, question
> body

```changes
+ added.go
~ edited.go  what changed
- removed.go
> old.go -> new.go
```

![caption](shot.png)       an image, inlined into the page

```go
code                       a code block
```
````

`###` inside a section is a small subheading.

## The graph and the decision log

A `graph` document is an edge list, one arrow per line: `web --> api`. Node labels are
`api [API server]`, edge labels are `api --> queue : async`. Do not draw the graph by
hand; write the edges and let the page lay them out.

For a folder of ADRs or specs, do not build an index by hand: `vistan docs <path>`
renders each record and writes the index by status, with the counts. Use it after adding
or editing a record.

## The review workflow

1. Send N reviewers at the change (correctness, security, tests, readability).
2. Collect their findings into one `review.md`: one section per finding, named
   `F1 · Blocker · path/to/file.go:42`.
3. Put the test commands and their real results in `## Tests`, as checkboxes.
4. Put disagreements and gaps in `## Notes`.
5. `vistan new review > review.md`, fill it, `vistan serve review.md`.
6. The human ticks the findings to apply and copies the decision line.

Never invent a finding you did not get from a reviewer, and never mark a test as
passed when it was not run.

## Exit codes

`0` the document is complete, `1` a required section is missing or empty, `2` the file
cannot be written. Check the code before you tell the human the page is ready.

## Do not

- Paste the whole conversation, a raw diff or a log into the page.
- Invent sections the template does not have.
- Claim a check passed without running it.
- Leave a page with empty sections; say what you did not do instead.
