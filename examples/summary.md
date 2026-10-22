<!-- vistan: summary -->

# Summary: maps, figures and an index

Seven templates, a treemap for `map`, and one command to find what you generated.

## What changed

- `vistan map` now draws a squarified SVG treemap above the tree: area is size, colour follows the parent.
- `selection` options can carry figures, code and prose, not just bullets.
- `diagram`, `adr` and `changelog` got fixed sections: context/reading, options and links, one block per release.
- `vistan index` lists every page with its link and template name.
- `###` subheads render, and bold that wraps code no longer leaks its asterisks.

## Verification

- `go test ./...`: 19 checks, all green. `go vet` and `gofmt -l` clean.
- The seven example pages answer 200 and were read in the browser at 980px, light and dark.
- `vistan map .` on this repo: 27 files, 2702 lines, treemap labels inside their boxes.

## Pending

- Push the five commits.
- Write the Homebrew formula once there are release tarballs.
