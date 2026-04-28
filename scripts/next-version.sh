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

last_tag=$(git tag --list 'v[0-9]*' --sort=-version:refname | head -n 1)
if [ -z "$last_tag" ]; then
  last_tag="v0.0.0"
fi

version=${last_tag#v}
old_ifs=$IFS
IFS=.
set -- $version
IFS=$old_ifs
major=${1:-0}
minor=${2:-0}
patch=${3:-0}

case "$kind" in
  patch)
    patch=$((patch + 1))
    ;;
  minor)
    minor=$((minor + 1))
    patch=0
    ;;
  major)
    major=$((major + 1))
    minor=0
    patch=0
    ;;
esac

printf 'v%s.%s.%s\n' "$major" "$minor" "$patch"
