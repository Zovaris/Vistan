<!-- vistan: changelog -->

# Changelog

## Unreleased

### Added

- `vistan index` lists every generated page with its link, and serves them from one address.
- `map` renders a treemap of directory sizes above the tree.

### Changed

- `selection` options can carry an image, a code snippet and a figure, not just bullets.

## 0.2.0 — 20 Oct 2026

### Added

- `vistan map <path>` scans any directory and renders a tree, the language mix and the entry points.
- Templates `adr` and `changelog`.

### Changed

- The page moved from a dark dashboard to a paper document: serif body, 38rem measure, hairline rules.
- `vistan serve` starts a detached local server instead of opening a `file://` URL.

### Fixed

- The server died with the command that started it, because it inherited the process group.
- Nested list levels were computed from the regex match position instead of its length.

## 0.1.0 — 19 Oct 2026

### Added

- First release: `plan`, `summary`, `selection`, `map` and `diagram`.

### Removed

- The bash and awk implementation, replaced by the Go binary.
