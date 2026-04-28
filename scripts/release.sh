#!/bin/sh
set -eu

kind=${1:-}
if [ -z "$kind" ]; then
  echo "usage: $0 <patch|minor|major>" >&2
  exit 1
fi

case "$kind" in
  patch|minor|major) ;;
  *)
    echo "invalid release kind: $kind" >&2
    exit 1
    ;;
esac

if ! git diff --quiet --ignore-submodules -- || ! git diff --cached --quiet --ignore-submodules --; then
  echo "working tree must be clean before creating a release tag" >&2
  exit 1
fi

next_version=$(./scripts/next-version.sh "$kind")

echo "Creating release tag $next_version"
git tag -a "$next_version" -m "Release $next_version"
git push origin "$next_version"

echo "Released $next_version"
