<!-- vistan: summary -->

# Summary: vistan v1

A shell script that turns a Markdown document into a page a human can read without scrolling a chat.

## What changed

- `vistan serve` builds the page, starts a local server and opens it.
- Five templates: plan, summary, selection, map and diagram.
- The selection template adds radios, an answer field and a copy button.
- The page moved from a dark dashboard to a paper document: serif, 38rem, 1px rules.

## Verification

- `./test.sh`: 14 checks, all green.
- The served page answers 200 and contains the four plan sections.
- The HTML escapes `<script>` and quotes inside radio values.

## Pending

- Decide whether the page should be reachable from other machines.
- Try the copy button in Safari, not just Chromium.
