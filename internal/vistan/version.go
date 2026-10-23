package vistan

import (
	"runtime/debug"
	"strings"
)

// version is replaced at build time from the git tag:
//
//	go build -ldflags "-X github.com/zovaris/vistan/internal/vistan.version=0.1.0"
//
// goreleaser does that on every release, so a tagged build always knows its version.
var version = "dev"

// bareVersion is the one place that knows the tag carries a v and the binary does not:
// every version a command prints, compares or puts in a file name comes through here.
func bareVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}

// versionFromBuildInfo keeps the module version of a tagged `go install ...@v0.1.0`
// and throws away the pseudo-version a build from a checkout records.
func versionFromBuildInfo(v string) string {
	if strings.HasPrefix(v, "v") && !strings.ContainsAny(v, "-+") {
		return strings.TrimPrefix(v, "v")
	}
	return ""
}

// versionString prefers the injected version and falls back to that module version, so
// an installed release reports its tag with no flags at all.
func versionString() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := versionFromBuildInfo(info.Main.Version); v != "" {
			return v
		}
	}
	return version
}
