<!-- vistan: diagram -->

# What vistan receives

## Context

One Markdown file and nothing else: no JSON, no TOML, no TOON, no schema to compile. What makes it a vistan document is the shape — a marker that picks the template, one `#` title, and `##` sections whose names the template expects. The renderer reads that shape and ignores the file extension.

## Diagram

```
          plan.md
          ┌───────────────────────┐
          │ <!-- vistan: plan --> │
          │ # Migrate the cache   │
          │                       │
          │ Intro paragraph.      │
          │                       │
          │ ## Decisions          │
          │ - keep the markdown   │
          │ - drop the database   │
          │                       │
          │ ## Verification       │
          │ - [x] go test ./...   │
          └───────────┬───────────┘
                      │ Parse()
                      ▼
        Doc{Title, Intro, Sections}
           │                    │
           ▼                    ▼
       terminal             renderHTML
       exit 0 or 1          one page on :8765
```

## Reading

- The marker is optional: `<!-- vistan: review -->` on its own line picks the shape, and without one the document is a `plan`.
- `# ` is the title, the first one wins. Every `## ` starts a section, and the section name is the key: `Decisions`, `Verification`, `F1 · Blocker · cache.go:42`. That is the whole schema.
- Text before the first `##` is the intro; text after a section belongs to it, until the next one.
- A template lists the sections it needs. `vistan file.md` exits `1` when one is missing or empty; `serve` still renders the page and puts the warning in the footer.
- Section names are case-sensitive: `Your move`, not `Your Move`.
- Inside a section the subset is deliberate: bullets, `- [x]` tasks, `###` subheads, `>` callouts tagged `[!WARN]`, fenced code, a `changes` fence, inline code, bold, links and images.
- `map` takes no document at all: it scans a directory. `docs` takes a folder of ADRs and specs. `history` reads the pages vistan already wrote.

## What it does not read

- **JSON** — `{` and `}` are ordinary characters. A `.json` file parses as a document with no title and no sections, so every line becomes a paragraph.
- **TOML** — a `[table]` line is text, not a section. Same for `key = value`.
- **TOON** — the extension means nothing; only the marker, the two heading levels and the fences matter.
- **YAML front matter** — `---` is not a delimiter, so those lines land in the intro and render as text.
- **Tables and ordered lists** — not implemented: pipes and `1.` render literally.

```json
{
  "title": "Migrate the cache",
  "sections": ["Decisions", "Verification"]
}
```

That block is shown as code because it sits in a fence. The same four lines outside a fence become four paragraphs, which is what a JSON document does when you hand it to vistan.

## The ten shapes

- `plan` — Decisions, Rejected, Verification, Your move.
- `summary` — What changed, Verification, Pending.
- `selection` — one section per option, no required names.
- `review` — one section per finding, `F1 · Blocker · path:42`, plus Tests and Notes.
- `map` — Overview, Git, Tree, Entry points, Largest files; `vistan map` fills it.
- `diagram` — Context, Diagram, Reading.
- `graph` — Context, Graph, Reading; the Graph section is an edge list, not a picture.
- `adr` — Status, Context, Decision, Consequences.
- `spec` — Status, Problem, Proposal, Decisions.
- `changelog` — one section per release, no required names.

There is no version field, no schema file and no validation beyond "are these sections present". The shape is the contract, and the exit code is the check.
