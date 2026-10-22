<!-- vistan: spec -->

# Spec: keep generated pages between runs

## Status

accepted — 22 Oct 2026

## Problem

Every page used to land in `$TMPDIR/vistan`, so the machine's temp cleanup decided how long a plan stayed readable. A review you wanted to reopen the next morning was gone, and there was no way to see what had been generated yesterday.

## Proposal

Move the pages to the user state directory, keep the last 50 by default, and derive an index from whatever is on disk. No database: the files are the record, and the index is regenerated on every read.

`VISTAN_KEEP` sets the limit, `VISTAN_CACHE` still overrides the directory.

## Decisions

- **State directory, not the project** — pages outlive a checkout and never appear in `git status`. The cost is that they are not shared with the team.
- **Number of pages, not age** — a quiet week should not delete the only plan you have. The cost is that one busy afternoon can push out a review from last month.
- **Prune after writing** — the new page always survives its own render, and `index.html` is exempt, so the index never links to a deleted file.
- **Index derived, not stored** — no drift between the list and the files. The cost is a directory read per command.

## Open questions

- Whether retention should be per project, so a noisy repository cannot evict another one's pages.
- Whether `vistan rm` should rewrite the index or let the next `history` do it.

## Links

- [index.go](../index.go)
- [main.go](../main.go)
