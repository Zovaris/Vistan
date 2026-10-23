#!/bin/sh
# Install the latest Vistan release (or the tag you pass) from GitHub.
#   curl -fsSL https://raw.githubusercontent.com/zovaris/vistan/main/install.sh | sh
#   sh install.sh v0.1.0
# VISTAN_BIN_DIR overrides the install directory, which is /usr/local/bin, or
# ~/.local/bin when that is not writable.
set -eu

repo=zovaris/vistan
die() {
	echo "install.sh: $*" >&2
	exit 1
}

tag=${1:-}
if [ -z "$tag" ]; then
	url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest") ||
		die "could not reach GitHub"
	tag=${url##*/}
fi
case "$tag" in
v[0-9]*) ;;
[0-9]*) tag=v$tag ;;
*) die "no release found; pass a tag like v0.1.0" ;;
esac
version=${tag#v}

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
darwin | linux) ;;
*) die "$os is not supported; build it with: go install github.com/$repo/cmd/vistan@latest" ;;
esac
case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*) die "$(uname -m) is not supported; build it with: go install github.com/$repo/cmd/vistan@latest" ;;
esac

file="vistan_${version}_${os}_${arch}.tar.gz"
base="https://github.com/$repo/releases/download/$tag"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "fetching $file"
curl -fsSL -o "$tmp/$file" "$base/$file" || die "no archive for $os/$arch in $tag"
want=$(curl -fsSL "$base/checksums.txt" | awk -v f="$file" '$2 == f { print $1 }') ||
	die "could not read checksums.txt"
[ -n "$want" ] || die "$file is not in checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
	got=$(sha256sum "$tmp/$file" | awk '{ print $1 }')
else
	got=$(shasum -a 256 "$tmp/$file" | awk '{ print $1 }')
fi
[ "$got" = "$want" ] || die "checksum mismatch: expected $want, got $got"

tar -xzf "$tmp/$file" -C "$tmp" vistan || die "could not unpack $file"
bin=${VISTAN_BIN_DIR:-/usr/local/bin}
if [ -z "${VISTAN_BIN_DIR:-}" ] && [ ! -w "$bin" ]; then
	bin=$HOME/.local/bin
fi
mkdir -p "$bin" || die "cannot create $bin; set VISTAN_BIN_DIR"
install -m 755 "$tmp/vistan" "$bin/vistan" || die "cannot write $bin/vistan"

echo "vistan $version installed to $bin/vistan"
case ":$PATH:" in
*":$bin:"*) ;;
*) echo "add $bin to your PATH: export PATH=\"$bin:\$PATH\"" ;;
esac
