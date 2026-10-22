<!-- vistan: review -->

# Review: the treemap and the index

Three reviewers read the last two commits: one for correctness, one for the HTML the browser gets, one for what the server exposes. Verdict: two blockers, three smaller things, tests green.

## Tests

- [x] `go test ./...` — 25 checks, all pass
- [x] `go vet ./...` and `gofmt -l .` — clean
- [ ] `curl` against a cold cache — not run after the port change

## F1 · Blocker · `render.go:212`

`imageSrc` reads the image from disk and inlines it, but a missing file falls through to `src` unchanged. In `review` and `selection` that leaves a broken `<img>` with no hint of which page or file it came from.

**Fix:** keep the fallback, but add `alt` text that says the image is missing, and print the path to stderr so the agent can see it.

```go
if err != nil {
    fmt.Fprintf(os.Stderr, "missing image: %s\n", src)
    return html.EscapeString(src)
}
```

## F2 · Blocker · `server.go:64`

The server binds `0.0.0.0` when `VISTAN_HOST` is set, and there is no check that the value is loopback. A page with paths and code becomes reachable from the network by one typo.

**Fix:** refuse any host that is not `127.0.0.1` or `::1`, instead of trusting the environment.

## F3 · Major · `index.go:41`

Titles are clipped with `-34s` on bytes. A title in another script gets cut mid-rune and the terminal prints a replacement character.

**Fix:** count runes before padding, as the new `clip` helper already does for the page.

## F4 · Minor · `render.go:388`

`statusClass` matches the word anywhere in the section, so an ADR whose context says "we rejected caching" gets a red chip even when its status is `accepted`.

**Fix:** read only the first line of the Status section.

## F5 · Nit · `examples/diagram.md:12`

The diagram still labels the renderer as `awk (html)` from the shell version.

**Fix:** point it at `render.go` and `serve-child`.

## Notes

- The HTML reviewer did not test mobile widths below 360px.
- Reviewers disagreed on F3: one called it a nit, the other a major. It is kept as major because it corrupts output.
- Nothing here was checked against a release build.
