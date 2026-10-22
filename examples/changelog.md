<!-- vistan: changelog -->

# Changelog

## Added

- `vistan map <path>` scans any directory and renders a tree, the language mix and the entry points.
- Templates `adr` and `changelog`.
- `vistan serve` starts a detached local server instead of opening a `file://` URL.

## Changed

- The page moved from a dark dashboard to a paper document: serif body, 38rem measure, hairline rules.
- The `map` template now describes exactly the shape the scanner produces.

## Fixed

- The server died with the command that started it, because it inherited the process group.
- Nested list levels were computed from the regex match position instead of its length.

## Removed

- The bash and awk implementation, replaced by the Go binary.
