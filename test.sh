#!/usr/bin/env bash
# Checks for vistan. Usage: ./test.sh
set -uo pipefail
cd "$(dirname "$0")"
tmp=$(mktemp -d)
VISTAN_CACHE="$tmp/cache"
export VISTAN_CACHE
fail=0

cleanup() {
  ./vistan stop > /dev/null 2>&1 || true
  rm -rf "$tmp"
}
trap cleanup EXIT

ok() { printf 'ok   %s\n' "$1"; }
ko() { printf 'FAIL %s\n' "$1"; fail=1; }
check() { if [ "$1" -eq 0 ]; then ok "$2"; else ko "$2"; fi; }

# 1. A complete plan renders and exits 0.
./vistan examples/plan.md > "$tmp/a" 2>&1
rc=$?
[ $rc -eq 0 ] && grep -q 'plan complete' "$tmp/a"
check $? "plan complete"

# 2. A missing section exits 1 and is named.
awk '/^## Verification/{f=1; next} /^## /{f=0} !f' examples/plan.md > "$tmp/missing.md"
./vistan "$tmp/missing.md" > "$tmp/b" 2>&1
rc=$?
[ $rc -eq 1 ] && grep -q 'missing or empty:.*Verification' "$tmp/b"
check $? "missing section"

# 3. An empty section exits 1 and is named.
awk '/^## Rejected/{print; f=1; next} /^## /{f=0} !f' examples/plan.md > "$tmp/empty.md"
./vistan "$tmp/empty.md" > "$tmp/c" 2>&1
rc=$?
[ $rc -eq 1 ] && grep -q 'missing or empty:.*Rejected' "$tmp/c"
check $? "empty section"

# 4. No arguments: usage and exit 2.
./vistan > /dev/null 2>&1
[ $? -eq 2 ]
check $? "no arguments"

# 5. Piped output carries no ANSI codes.
./vistan examples/plan.md | grep -q $'\033'
[ $? -ne 0 ]
check $? "no colour outside tty"

# 6. serve answers 200 with the four sections.
port=$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()')
url=$(VISTAN_NO_OPEN=1 VISTAN_PORT="$port" ./vistan serve examples/plan.md 2>&1)
curl -fs "$url" > "$tmp/plan.html" 2>/dev/null
rc=$?
[ $rc -eq 0 ] && [ "$(grep -c '<section>' "$tmp/plan.html")" -eq 4 ] \
  && grep -q '<footer>Plan complete</footer>' "$tmp/plan.html"
check $? "serve: four sections"

# 7. selection: one radio per option plus "none", and the copy button.
sel=$(VISTAN_NO_OPEN=1 ./vistan serve examples/selection.md 2>&1)
curl -fs "$sel" > "$tmp/sel.html" 2>/dev/null
[ "$(grep -c 'type="radio"' "$tmp/sel.html")" -eq 5 ] && grep -q 'id="copy"' "$tmp/sel.html"
check $? "selection: radios and copy"

# 8. stdin works and HTML in the content is escaped.
printf '<!-- vistan: selection -->\n\n# Pick\n\n## <script>x</script> & y\n\n- z\n' \
  | VISTAN_NO_OPEN=1 ./vistan serve - > "$tmp/stdin.url" 2>&1
curl -fs "$(cat "$tmp/stdin.url")" > "$tmp/stdin.html" 2>/dev/null
grep -q '&lt;script&gt;x&lt;/script&gt;' "$tmp/stdin.html" && grep -q '&amp; y' "$tmp/stdin.html"
check $? "stdin and escaping"

# 9. summary validates its own three sections.
./vistan examples/summary.md > /dev/null 2>&1
full=$?
printf '<!-- vistan: summary -->\n\n# S\n\n## What changed\n- x\n' > "$tmp/partial.md"
./vistan "$tmp/partial.md" > /dev/null 2>&1
partial=$?
[ $full -eq 0 ] && [ $partial -eq 1 ]
check $? "summary complete and incomplete"

# 10. Unknown template exits 2.
./vistan --template nope examples/plan.md > /dev/null 2>&1
[ $? -eq 2 ]
check $? "unknown template"

# 11. new prints the marker so the document declares its own template.
./vistan new map | grep -q 'vistan: map'
check $? "new map"

# 12. templates lists all five.
[ "$(./vistan templates | wc -l | tr -d ' ')" -eq 5 ]
check $? "templates list"

# 13. The map page renders complete, with no footer warning.
VISTAN_NO_OPEN=1 ./vistan serve examples/map.md > "$tmp/map.url" 2>&1
curl -fs "$(cat "$tmp/map.url")" > "$tmp/map.html" 2>/dev/null
rc=$?
[ $rc -eq 0 ] && ! grep -q 'Missing or empty' "$tmp/map.html"
check $? "map complete"

# 14. stop shuts the server down.
./vistan stop > /dev/null 2>&1
curl -fs -o /dev/null "$(cat "$tmp/map.url")" 2>/dev/null
[ $? -ne 0 ]
check $? "stop server"

exit "$fail"
