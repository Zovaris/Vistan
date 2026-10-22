<!-- vistan: summary -->

# Summary: maps, figures and an index

Ten templates, a treemap for `map`, a graph drawn from an edge list, and one command to find what you generated.

## What changed

- `vistan map` now draws a squarified SVG treemap above the tree: area is size, colour follows the parent.
- `selection` options can carry figures, code and prose, not just bullets.
- `diagram`, `adr` and `changelog` got fixed sections: context/reading, options and links, one block per release.
- `vistan history` groups the saved pages by project, with a count and the last time each project wrote one; pages live in the user state directory and the oldest are pruned after 50.
- `vistan open <name>` reopens a page by name or prefix, and `vistan rm` deletes pages and rewrites the index.
- `graph` turns `web --> api` edge lists into an inline SVG with layers and dashed loops, no JavaScript and no coordinates to maintain.
- `vistan docs <path>` renders every ADR and spec under a directory and writes an index grouped by status, with kind, date and source.
- Plans render checkboxes with progress, labelled callouts, a file-change list and a `waiting on you` panel.
- `review` turns several agents' findings into cards with severity, location and a decision per finding.
- `vistan brief` prints the instructions the orchestrator can hand to a cheap sub-agent.
- `###` subheads render, and bold that wraps code no longer leaks its asterisks.

## Verification

- `go test ./...`: 40 checks, all green. `go vet` and `gofmt -l` clean.
- The ten example pages answer 200 and were read in the browser, light and dark.
- `vistan map .` on this repo: treemap labels inside their boxes, and the tree shows the new `graph.go` and `docs.go`.

## Pending

- Push the commits.
- Write the Homebrew formula once there are release tarballs.
