#!/bin/sh
set -eu

build_number=${1:?build number is required}
case "$build_number" in
  *[!A-Za-z0-9._-]*|'')
    echo "build number is unsafe" >&2
    exit 1
    ;;
esac

source_sha=${PINNED_SOURCE_SHA:-}
if [ -z "$source_sha" ]; then
  source_sha=$(git rev-parse HEAD)
fi
case "$source_sha" in
  [0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]) ;;
  *)
    echo "source checkout is not a full commit" >&2
    exit 1
    ;;
esac

if [ -z "${PINNED_SOURCE_SHA:-}" ]; then
  git fetch --no-tags origin cce-deploy
  test "$(git rev-parse 'FETCH_HEAD^{commit}')" = "$source_sha" || {
    echo "cce-deploy moved while the release source was being pinned" >&2
    exit 1
  }
  git merge-base --is-ancestor "$source_sha" FETCH_HEAD
fi

base_version=$(tr -d '\r\n' < backend/cmd/server/VERSION)
case "$base_version" in
  *[!A-Za-z0-9._-]*|'')
    echo "runtime version is unsafe" >&2
    exit 1
    ;;
esac
source_version="${base_version}-cce-codearts-${build_number}"
source_date=${PINNED_SOURCE_DATE:-}
if [ -z "$source_date" ]; then
  if [ -n "${PINNED_SOURCE_SHA:-}" ]; then
    source_date=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  elif git rev-parse --git-dir >/dev/null 2>&1; then
    source_date=$(git show -s --format=%cI "$source_sha")
  else
    source_date=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  fi
fi

metadata=deploy/cce/codearts/build-metadata.env
umask 022
{
  printf 'SOURCE_SHA=%s\n' "$source_sha"
  printf 'SOURCE_VERSION=%s\n' "$source_version"
  printf 'SOURCE_DATE=%s\n' "$source_date"
} > "$metadata"

python3 deploy/cce/stage_setup_runtime.py

printf 'Pinned cce-deploy source %s as %s\n' "$source_sha" "$source_version"
