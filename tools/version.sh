#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BUMP=patch
TARGET=""
DRY=0
PUSH=1

usage() {
  cat <<MSG
usage: tools/version.sh [--bump patch|minor|major] [--version X.Y.Z] [--dry-run] [--no-push]

  --bump      which number to raise when --version is not given (default patch)
  --version   the exact version to release
  --dry-run   print the plan and change nothing
  --no-push   create the tag locally, do not push
MSG
}

normalize() {
  IFS=. read -r a b c <<< "$1"
  echo "${a:-0}.${b:-0}.${c:-0}"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --bump) BUMP="${2:?}"; shift 2 ;;
    --version) TARGET="${2:?}"; shift 2 ;;
    --dry-run) DRY=1; shift ;;
    --no-push) PUSH=0; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1"; usage; exit 2 ;;
  esac
done

current="$(git -C "$ROOT" describe --tags --abbrev=0 2>/dev/null || true)"
current="${current#v}"
current="$(normalize "${current:-0.0.0}")"

if [ -n "$TARGET" ]; then
  # the tag carries the v, the number does not: accept either spelling here
  TARGET="${TARGET#v}"
  [[ "$TARGET" =~ ^[0-9]+(\.[0-9]+){0,2}$ ]] || { echo "not a version: $TARGET (expected X.Y.Z)"; exit 2; }
  next="$(normalize "$TARGET")"
else
  IFS=. read -r major minor patch <<< "$current"
  case "$BUMP" in
    patch) patch=$((patch + 1)) ;;
    minor) minor=$((minor + 1)); patch=0 ;;
    major) major=$((major + 1)); minor=0; patch=0 ;;
    *) echo "--bump takes patch, minor or major (got $BUMP)"; exit 2 ;;
  esac
  next="$major.$minor.$patch"
fi

[ "$next" != "$current" ] || { echo "the version is already $current"; exit 2; }

tag="v$next"
branch="$(git -C "$ROOT" rev-parse --abbrev-ref HEAD)"

if [ "$DRY" = "1" ]; then
  echo "version: $current -> $next"
  echo "tag:     $tag (annotated)"
  echo "push:    $([ "$PUSH" = "1" ] && echo "origin $branch and $tag" || echo no)"
  exit 0
fi

[ "$branch" = "main" ] || { echo "release from main, not $branch"; exit 1; }
if ! git -C "$ROOT" diff --quiet HEAD; then
  echo "the working tree has uncommitted changes; commit or stash them first"
  git -C "$ROOT" status --short
  exit 1
fi
if git -C "$ROOT" rev-parse -q --verify "refs/tags/$tag" > /dev/null; then
  echo "tag $tag already exists"
  exit 1
fi

git -C "$ROOT" tag -a "$tag" -m "vistan $tag"
echo "tagged $tag at $(git -C "$ROOT" rev-parse --short HEAD); the tag is the version, no file needed a bump"

if [ "$PUSH" = "1" ]; then
  git -C "$ROOT" push origin "$branch"
  git -C "$ROOT" push origin "$tag"
  echo "pushed $branch and $tag; the release workflow builds the binaries from there"
else
  echo "not pushed: run 'git push origin $tag' when you are ready"
fi
