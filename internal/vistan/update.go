package vistan

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// releasesURL is the page goreleaser publishes to; /latest redirects to the newest tag.
const releasesURL = "https://github.com/zovaris/vistan/releases"

// runUpdate replaces this binary with the latest release, or reports that there is
// nothing to do. It never guesses: the archive has to match the release's checksum.
func runUpdate() int {
	current := versionString()
	if current == "dev" {
		fmt.Fprintln(os.Stderr, "this build has no version; install a release with:")
		fmt.Fprintln(os.Stderr, "  curl -fsSL https://raw.githubusercontent.com/zovaris/vistan/main/install.sh | sh")
		return 1
	}

	tag, err := latestTag()
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not read the latest release: %v\n", err)
		return 1
	}
	if bareVersion(tag) == current {
		fmt.Printf("vistan %s is the latest release\n", current)
		return 0
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot locate this binary: %v\n", err)
		return 1
	}
	if strings.Contains(exe, "/Cellar/") {
		fmt.Printf("vistan %s comes from Homebrew; run: brew upgrade vistan\n", current)
		return 0
	}
	if err := installRelease(tag, exe); err != nil {
		fmt.Fprintf(os.Stderr, "update failed: %v\n", err)
		return 1
	}
	fmt.Printf("vistan %s -> %s in %s\n", current, bareVersion(tag), exe)
	return 0
}

// latestTag follows the /latest redirect instead of the API, which needs no token and
// counts against no rate limit.
func latestTag() (string, error) {
	client := &http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Get(releasesURL + "/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("%s answered %s", releasesURL+"/latest", resp.Status)
	}
	tag := path.Base(strings.TrimSuffix(loc, "/"))
	if !strings.HasPrefix(tag, "v") {
		return "", fmt.Errorf("no tag in %s", loc)
	}
	return tag, nil
}

func installRelease(tag, exe string) error {
	name := assetName(tag, runtime.GOOS, runtime.GOARCH)
	if name == "" {
		return fmt.Errorf("no release archive for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	base := releasesURL + "/download/" + tag
	archive, err := fetch(base + "/" + name)
	if err != nil {
		return err
	}
	sums, err := fetch(base + "/checksums.txt")
	if err != nil {
		return err
	}
	want := checksumFor(string(sums), name)
	if want == "" {
		return fmt.Errorf("%s is missing from checksums.txt", name)
	}
	sum := sha256.Sum256(archive)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("checksum mismatch for %s", name)
	}
	binary, err := binaryFromTarGz(archive, "vistan")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(exe), ".vistan-update-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(binary); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), exe)
}

// assetName mirrors the goreleaser template, and answers "" for a platform without a
// release instead of asking GitHub for a 404.
func assetName(tag, goos, goarch string) string {
	if goos != "darwin" && goos != "linux" {
		return ""
	}
	return fmt.Sprintf("vistan_%s_%s_%s.tar.gz", bareVersion(tag), goos, goarch)
}

func checksumFor(sums, name string) string {
	for _, line := range strings.Split(sums, "\n") {
		if hash, file, ok := strings.Cut(strings.TrimSpace(line), "  "); ok && file == name {
			return hash
		}
	}
	return ""
}

func binaryFromTarGz(archive []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("%s is not in the archive", name)
		}
		if err != nil {
			return nil, err
		}
		if header.Name == name {
			return io.ReadAll(io.LimitReader(tr, 64<<20))
		}
	}
}

func fetch(url string) ([]byte, error) {
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}
