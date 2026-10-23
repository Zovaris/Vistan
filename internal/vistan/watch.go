package vistan

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// reloadScript polls the page it is in and reloads the tab when the file behind it
// changes. It is written only by serve --watch, so a normal page pays nothing.
const reloadScript = `<script>
(() => {
  let last = null;
  setInterval(async () => {
    try {
      const r = await fetch(location.href, { method: 'HEAD', cache: 'no-store' });
      const m = r.headers.get('last-modified');
      if (last && m !== last) location.reload();
      last = m;
    } catch {}
  }, 1000);
})();
</script>`

func withReload(page string) string {
	if i := strings.LastIndex(page, "</body>"); i >= 0 {
		return page[:i] + reloadScript + page[i:]
	}
	return page + reloadScript
}

// watchPage rebuilds the page every time the source file changes, until the process
// is stopped. Unlike serve, it does not return: the command stays in the foreground.
func watchPage(cache, name, source, baseDir string) {
	info, err := os.Stat(source)
	if err != nil {
		return
	}
	last := info.ModTime()
	for {
		time.Sleep(300 * time.Millisecond)
		info, err := os.Stat(source)
		if err != nil || !info.ModTime().After(last) {
			continue
		}
		last = info.ModTime()
		lines, err := readFileLines(source)
		if err != nil {
			continue
		}
		tplName := DeclaredTemplate(lines)
		if tplName == "" {
			tplName = "plan"
		}
		tpl, ok := lookupTemplate(tplName)
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown template: %s\n", tplName)
			continue
		}
		if err := writePage(cache, name, withReload(renderHTML(Parse(lines), tpl, source, baseDir)), lines); err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		refreshIndex(cache)
		fmt.Printf("rebuilt %s\n", name)
	}
}
