<!-- vistan: diagram -->

# How a document becomes a page

One Markdown file, two outputs. Both come out of the same awk parser.

```
  doc.md
    │
    ├──► awk (terminal) ──► stdout, colours off when piped
    │
    └──► awk (html)
            │
            └──► $TMPDIR/vistan/<name>.html
                    │
                    └──► python http.server :8765
                            │
                            └──► browser
```

## Notes

- The server is a plain directory server; it does not know about templates.
- Nothing is generated until `vistan` runs, so a stale tab keeps the old page.
