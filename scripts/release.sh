#!/usr/bin/env bash
# Release helper: bump VERSION (+ web/package.json, web/package-lock.json),
# commit, and tag.
#
# Usage:
#   scripts/release.sh <major|minor|patch>
#
# Bumps the semver in VERSION, propagates it to the web package files, creates
# a single commit "Release version vX.X.X", and an annotated tag vX.X.X with
# the same message. Does not push — run `git push --follow-tags` when ready.

set -euo pipefail

usage() {
  echo "Usage: $(basename "$0") <major|minor|patch>" >&2
  exit 2
}

[ $# -eq 1 ] || usage
part="$1"
case "$part" in
  major | minor | patch) ;;
  *) usage ;;
esac

command -v npm >/dev/null 2>&1 || {
  echo "error: npm is required but not installed" >&2
  exit 1
}

repo_root=$(cd "$(dirname "$0")/.." && pwd)
cd "$repo_root"

version_file="VERSION"
pkg="web/package.json"
lock="web/package-lock.json"

[ -f "$version_file" ] || {
  echo "error: $version_file not found" >&2
  exit 1
}

current=$(tr -d '[:space:]' <"$version_file")
if ! [[ "$current" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
  echo "error: VERSION '$current' is not a X.Y.Z semver" >&2
  exit 1
fi
major="${BASH_REMATCH[1]}"
minor="${BASH_REMATCH[2]}"
patch="${BASH_REMATCH[3]}"

case "$part" in
  major) major=$((major + 1)); minor=0; patch=0 ;;
  minor) minor=$((minor + 1)); patch=0 ;;
  patch) patch=$((patch + 1)) ;;
esac

new="${major}.${minor}.${patch}"
tag="v${new}"
msg="Release version ${tag}"

printf 'Release v%s → %s? This commits and tags locally. [y/N] ' "$current" "$tag"
read -r reply </dev/tty
case "$reply" in
  y | Y) ;;
  *) echo "Aborted."; exit 1 ;;
esac

printf '%s\n' "$new" >"$version_file"

# Let npm own package.json + package-lock.json so the lockfile stays consistent.
(cd web && npm version "$new" --no-git-tag-version --allow-same-version >/dev/null)

git add "$version_file" "$pkg" "$lock"
git commit -m "$msg"
git tag -a "$tag" -m "$msg"

echo "Committed and tagged $tag."
echo "Push with: git push --follow-tags"
