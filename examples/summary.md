<!-- vistan: summary -->

# Summary: maps, figures and an index

Eight templates, a treemap for `map`, and one command to find what you generated.

## What changed

- `vistan map` now draws a squarified SVG treemap above the tree: area is size, colour follows the parent.
- `selection` options can carry figures, code and prose, not just bullets.
- `diagram`, `adr` and `changelog` got fixed sections: context/reading, options and links, one block per release.
- `vistan index` lists every page with its link and template name.
- Plans render checkboxes with progress, labelled callouts, a file-change list and a `waiting on you` panel.
- `review` turns several agents' findings into cards with severity, location and a decision per finding.
- `vistan brief` prints the instructions the orchestrator can hand to a cheap sub-agent.
- `###` subheads render, and bold that wraps code no longer leaks its asterisks.

## Verification

- `go test ./...`: 19 checks, all green. `go vet` and `gofmt -l` clean.
- The seven example pages answer 200 and were read in the browser at 980px, light and dark.
- `vistan map .` on this repo: 27 files, 2702 lines, treemap labels inside their boxes.

## Pending

- Push the commits once the review is done.
- Write the Homebrew formula once there are release tarballs.
