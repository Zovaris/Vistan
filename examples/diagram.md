<!-- vistan: diagram -->

# How a document becomes a page

## Context

One Markdown file, two outputs: a coloured terminal summary for the agent, and a page for the browser. The diagram shows where the parser sits and what the server does and does not know.

## Diagram

```
                  ┌──────────────┐
   doc.md ───────►│   parser     │
                  │  (doc.go)    │
                  └──────┬───────┘
                         │  *Doc
        ┌────────────────┴────────────────┐
        │                                 │
        ▼                                 ▼
  ┌───────────┐                     ┌───────────┐
  │ terminal  │                     │   html    │
  │ render.go │                     │ render.go │
  └─────┬─────┘                     └─────┬─────┘
        │                                 │
        ▼                                 ▼
    stdout                      $TMPDIR/vistan/*.html
                                              │
                                              ▼
                                  server :8765 (serve-child)
                                              │
                                              ▼
                                          browser
```

## Reading

- Both renderers read the same `*Doc`; a shape that parses once cannot drift between them.
- The server is a plain file server: it does not know about templates, so editing a document means running `vistan` again.
- The dashed path is future work: a page that reports a choice back to the running process.

## Notes

- Nothing is generated until `vistan` runs, so an open tab keeps showing the old page.
- The terminal renderer colours only when stdout is a terminal.
