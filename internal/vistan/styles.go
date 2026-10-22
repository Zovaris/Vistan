package vistan

const faviconTag = `<link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' width='32' height='32' viewBox='0 0 32 32'><rect width='32' height='32' rx='7' fill='%23f6f3ea'/><text x='16' y='23' font-family='Georgia,serif' font-size='20' text-anchor='middle' fill='%231b1a17'>v</text></svg>">` + "\n"
const pageCSS = `:root {
  color-scheme: light dark;
  --paper: #f6f3ea;
  --ink: #1b1a17;
  --muted: #625e55;
  --rule: #d2ccbf;
  --field: #fffdf6;
}
* { box-sizing: border-box; }
html { background: var(--paper); }
body { margin: 0; background: var(--paper); color: var(--ink);
  font-family: "Iowan Old Style", "Palatino Linotype", "URW Palladio L", P052, Palatino, Georgia, serif;
  font-size: 17px; line-height: 1.6; -webkit-font-smoothing: antialiased; }
main { width: min(38rem, calc(100vw - 2.5rem)); margin: 0 auto; padding: 80px 0 96px; }
h1 { margin: 0 0 8px; font-size: 2.05rem; line-height: 1.16; font-weight: 400;
  letter-spacing: -0.015em; text-wrap: balance; }
.src { margin: 0; color: var(--muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 0.72rem; }
section { margin-top: 32px; padding-top: 20px; border-top: 1px solid var(--rule); }
h2 { display: flex; align-items: baseline; justify-content: space-between; gap: 1em;
  margin: 0 0 10px; font-size: 1.05rem; font-style: italic; font-weight: 400; }
h2 span { font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif;
  font-size: 0.7rem; font-style: normal; letter-spacing: 0.05em; color: var(--muted);
  font-variant-numeric: tabular-nums; }
ul { margin: 4px 0 0; padding-left: 1.2em; }
li { margin: 0 0 8px; margin-left: calc(var(--l, 0) * 1.2em); }
li:last-child { margin-bottom: 0; }
li::marker { color: var(--muted); }
p { margin: 12px 0; }
code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 0.85em;
  background: #ece5d5; border-radius: 2px; padding: 0.1em 0.3em; }
pre { margin: 14px 0; padding: 14px 16px; overflow-x: auto; background: #efe9da;
  border: 1px solid var(--rule); border-radius: 3px; }
pre code { background: none; padding: 0; font-size: 0.8rem; line-height: 1.5; }
.empty { margin: 0; color: var(--muted); font-style: italic; }

.stats { display: flex; flex-wrap: wrap; gap: 1.3em; margin: 26px 0 0;
  font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif;
  font-size: 0.73rem; letter-spacing: 0.02em; color: var(--muted); }
.stats b { color: var(--ink); font-weight: 500; font-variant-numeric: tabular-nums; }
.stats .wait { color: var(--ink); border-bottom: 1px solid var(--rule); padding-bottom: 1px; }

li.task { list-style: none; position: relative; padding-left: 1.5em;
  margin-left: calc(var(--l, 0) * 1.2em - 1.15em); }
li.task .box { position: absolute; left: 0; top: 0.28em; width: 1.02em; height: 1.02em;
  display: inline-flex; align-items: center; justify-content: center;
  border: 1px solid var(--rule); border-radius: 3px; font-size: 0.66em;
  color: transparent; font-family: ui-sans-serif, sans-serif; }
li.task.done .box { background: var(--ink); border-color: var(--ink); color: var(--paper); }
li.task.done .txt { color: var(--muted); text-decoration: line-through;
  text-decoration-color: var(--rule); }

ul.changes { list-style: none; margin: 12px 0; padding: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 0.8rem; }
ul.changes li { display: flex; align-items: baseline; gap: 0.6em; margin: 0 0 5px;
  margin-left: 0; }
ul.changes .mark { flex: none; width: 1.15em; text-align: center; font-size: 0.68rem;
  font-weight: 600; color: var(--muted); }
ul.changes .add .mark { color: #3f7a4d; }
ul.changes .del .mark { color: #a1503f; }
ul.changes .move .mark { color: #5d6f8c; }
ul.changes code { background: none; padding: 0; white-space: pre-wrap; word-break: break-word; }
ul.changes .note { color: var(--muted); font-family: inherit; font-size: 0.78rem; }

aside.callout { margin: 18px 0; padding: 12px 16px 14px; border-left: 2px solid var(--rule);
  background: #efe9da; border-radius: 0 3px 3px 0; }
aside.callout .kind { margin: 0 0 5px; font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif;
  font-size: 0.62rem; letter-spacing: 0.1em; text-transform: uppercase; color: var(--muted); }
aside.callout .lead { margin: 0 0 4px; font-weight: 600; }
aside.callout p { margin: 6px 0; }
aside.callout p:last-child, aside.callout ul:last-child { margin-bottom: 0; }
aside.callout ul { margin: 4px 0 0; }
aside.callout.tip { background: #e9f0e4; border-left-color: #8ca57e; }
aside.callout.warn, aside.callout.risk { background: #f6e9dd; border-left-color: #c08a5e; }
aside.callout.decision { background: #e8ecf3; border-left-color: #8496b4; }
aside.callout.question { background: #f3eef7; border-left-color: #a58ab8; }

.tpl-review .finding { margin-top: 22px; padding: 16px 0 2px 16px; border-left: 2px solid var(--rule); }
.tpl-review .fhead { display: flex; align-items: baseline; flex-wrap: wrap; gap: 0.7em; }
.tpl-review .apply { display: flex; align-items: center; gap: 0.45em; cursor: pointer;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 0.78rem; color: var(--muted); }
.tpl-review .apply input { margin: 0; accent-color: var(--ink); }
.tpl-review .sev { font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif;
  font-size: 0.61rem; letter-spacing: 0.08em; text-transform: uppercase; padding: 1px 8px;
  border: 1px solid var(--rule); border-radius: 999px; color: var(--muted); }
.tpl-review .loc { margin-left: auto; background: none; padding: 0; font-size: 0.74rem; color: var(--muted); }
.tpl-review .fbody { margin-top: 8px; }
.tpl-review .fbody > :first-child { margin-top: 0; }
.tpl-review .sev-blocker, .tpl-review .sev-critical { border-left-color: #b4553c; }
.tpl-review .sev-blocker .sev, .tpl-review .sev-critical .sev { color: #8a3b2c; border-color: #dcc0b6; background: #f6e9e2; }
.tpl-review .sev-major, .tpl-review .sev-important { border-left-color: #c08a5e; }
.tpl-review .sev-major .sev, .tpl-review .sev-important .sev { color: #7a5620; border-color: #ddcbaa; background: #f6efdf; }
.tpl-review .sev-minor { border-left-color: #8496b4; }
.tpl-review .sev-minor .sev { color: #46586f; border-color: #c3ccdb; background: #eaeef4; }
.tpl-review .sev-question { border-left-color: #a58ab8; }
.tpl-review .sev-question .sev { color: #5d4470; border-color: #d3c3dd; background: #f2ecf6; }
.tpl-review section.decision { margin-top: 42px; padding: 18px 24px 20px; border: 1px solid var(--rule);
  border-radius: 4px; background: var(--field); }
.tpl-review section.decision .hint { margin: 0 0 12px; }
.tpl-review section.decision::before { content: "your decision"; display: block; margin-bottom: 8px;
  font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif; font-size: 0.62rem;
  letter-spacing: 0.09em; text-transform: uppercase; color: var(--muted); }

.tpl-plan section.move { margin-top: 42px; padding: 20px 24px 22px; border: 1px solid var(--rule);
  border-radius: 4px; background: var(--field); }
.tpl-plan section.move h2::after { content: "waiting on you"; font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif;
  font-size: 0.62rem; font-style: normal; letter-spacing: 0.09em; text-transform: uppercase;
  color: var(--muted); }

footer { margin-top: 40px; padding-top: 16px; border-top: 1px solid var(--rule);
  font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif;
  font-size: 0.75rem; letter-spacing: 0.02em; color: var(--muted); }
footer .warn { color: #8a4b1f; }

fieldset.opts { border: 0; padding: 0; margin: 36px 0 0; }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px;
  overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; border: 0; }
.opt .pick { display: flex; align-items: baseline; gap: 0.7rem; cursor: pointer; }
.opt .pick input { margin: 0; accent-color: var(--ink); }
.opt .key { font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.78rem; color: var(--muted); min-width: 1.1em; }
.opt .ttl { font-size: 1.05rem; }
.opt .desc { margin: 6px 0 0 2.1rem; }
.opt .desc > :first-child { margin-top: 0; }
.opt .desc > :last-child { margin-bottom: 0; }
.opt:has(input:checked) { border-top-color: var(--ink); }
.answer { display: flex; gap: 10px; margin-top: 30px; align-items: stretch; }
.answer input { flex: 1; min-width: 0; font: inherit; font-size: 0.9rem; padding: 8px 11px;
  border: 1px solid var(--rule); border-radius: 3px; background: var(--field); color: var(--ink); }
.answer button { font: inherit; font-size: 0.85rem; padding: 8px 15px; cursor: pointer;
  border: 1px solid var(--ink); border-radius: 3px; background: var(--ink); color: var(--paper); }
.answer button:disabled { opacity: 0.35; cursor: default; }
.hint { margin: 10px 0 0; color: var(--muted); font-size: 0.78rem;
  font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif; }

h3 { margin: 20px 0 6px; font-size: 0.72rem; font-weight: 500; letter-spacing: 0.08em;
  text-transform: uppercase; color: var(--muted);
  font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif; }
h3:first-child { margin-top: 4px; }
.art { margin: 22px 0 8px; }
.art svg { display: block; width: 100%; height: auto; }
.art text { font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  fill: var(--ink); fill-opacity: 0.78; }
.art text.sub { fill-opacity: 0.42; }
.art .tm rect { stroke: var(--paper); stroke-width: 1.5; }
.art .tm-c0 rect { fill: #b4713d; }
.art .tm-c1 rect { fill: #7f8f56; }
.art .tm-c2 rect { fill: #6d7f9c; }
.art .tm-c3 rect { fill: #a76a80; }
.art .tm-c4 rect { fill: #b39445; }
.art .tm-c5 rect { fill: #5e8c85; }
.art .tm-d0 rect { fill-opacity: 0.3; }
.art .tm-d1 rect { fill-opacity: 0.16; }
figure { margin: 16px 0; }
figure img { display: block; max-width: 100%; height: auto;
  border: 1px solid var(--rule); border-radius: 3px; }
figcaption { margin-top: 6px; color: var(--muted); font-size: 0.75rem;
  font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif; }
.opt .desc figure { margin: 12px 0; }
.tpl-diagram main { width: min(46rem, calc(100vw - 2.5rem)); }
.tpl-diagram pre { font-size: 0.92rem; padding: 22px 24px; line-height: 1.35;
  background-color: transparent; border-radius: 4px;
  background-image: radial-gradient(circle, var(--rule) 1px, transparent 1px);
  background-size: 16px 16px; background-position: 8px 8px; }
.tpl-diagram section:first-of-type pre { padding: 30px 28px; }

.back { display: inline-block; margin-bottom: 16px; text-decoration: none;
  font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif; font-size: 0.62rem;
  letter-spacing: 0.09em; text-transform: uppercase; color: var(--muted); }
.back:hover { color: var(--ink); }

.tpl-graph main { width: min(52rem, calc(100vw - 2.5rem)); }
.tpl-graph section.drawing { border-top: 0; padding-top: 8px; margin-top: 12px; }
.art .graph .node rect { fill: var(--field); stroke: var(--rule); stroke-width: 1; }
.art .graph .node text { font-size: 11px; fill: var(--ink); fill-opacity: 0.85; }
.art .graph .edge { fill: none; stroke: var(--muted); stroke-width: 1.1; stroke-opacity: 0.55; }
.art .graph .edge.back { stroke-dasharray: 4 4; }
.art .graph .arrowhead { fill: var(--muted); fill-opacity: 0.7; }
.art .graph .edge-label { font-size: 9px; fill: var(--muted); fill-opacity: 0.8; }

.tpl-adr section.status { border-top: 0; margin-top: 14px; padding-top: 0; }
.tpl-adr section.status h2 { display: none; }
.tpl-adr section.status p { display: inline-block; margin: 0; padding: 2px 11px;
  border: 1px solid var(--rule); border-radius: 999px; font-size: 0.78rem; color: var(--muted);
  font-family: ui-sans-serif, -apple-system, "Segoe UI", sans-serif; }
.tpl-adr section.status strong { font-weight: 500; }
.tpl-adr section.status.is-accepted p { color: #33623f; border-color: #b6cdb9; background: #eaf1e6; }
.tpl-adr section.status.is-rejected p { color: #8a3b2c; border-color: #dcc0b6; background: #f6e9e2; }
.tpl-adr section.status.is-proposed p { color: #7a5620; border-color: #ddcbaa; background: #f6efdf; }
.tpl-adr section.status.is-superseded p,
.tpl-adr section.status.is-deprecated p { text-decoration: line-through; }
.tpl-adr .record h2 { font-style: normal; }

.tpl-changelog h2 { font-style: normal; font-size: 1.15rem;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: -0.01em; }
.tpl-changelog .release.is-unreleased h2 { color: var(--muted); }
.tpl-changelog ul { margin-top: 0; }

@media (max-width: 560px) { main { padding: 48px 0 64px; } h1 { font-size: 1.7rem; } }
@media (prefers-color-scheme: dark) {
  :root { --paper: #13120e; --ink: #e7e1d3; --muted: #a49c88; --rule: #2a2720; --field: #1a1813; }
  .art .tm-d0 rect { fill-opacity: 0.5; }
  .art .tm-d1 rect { fill-opacity: 0.3; }
  aside.callout { background: #1c1a14; }
  aside.callout.tip { background: #171e16; }
  aside.callout.warn, aside.callout.risk { background: #221a14; }
  aside.callout.decision { background: #171b22; }
  aside.callout.question { background: #1d1822; }
  .tpl-review .sev-blocker .sev, .tpl-review .sev-critical .sev { color: #d8a295; border-color: #4a302a; background: #261a17; }
  .tpl-review .sev-major .sev, .tpl-review .sev-important .sev { color: #d6bd8a; border-color: #453a24; background: #221f16; }
  .tpl-review .sev-minor .sev { color: #a9bcd6; border-color: #2c3644; background: #171c24; }
  .tpl-review .sev-question .sev { color: #c3aed4; border-color: #3a2f44; background: #1f1a25; }
  ul.changes .add .mark { color: #8fbd95; }
  ul.changes .del .mark { color: #d29a88; }
  ul.changes .move .mark { color: #9aaecb; }
  .tpl-adr section.status.is-accepted p { color: #a8c8a8; border-color: #33452f; background: #1b241a; }
  .tpl-adr section.status.is-rejected p { color: #d8a295; border-color: #4a302a; background: #261a17; }
  .tpl-adr section.status.is-proposed p { color: #d6bd8a; border-color: #453a24; background: #221f16; }
  code { background: #221f18; }
  pre { background: #191712; }
  footer .warn { color: #d09a6a; }
  .answer button { background: var(--ink); color: #13120e; }
}
`
