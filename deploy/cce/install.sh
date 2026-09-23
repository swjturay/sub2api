#!/bin/sh
set -eu

source_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
destination=${SUB2API_CCE_RELEASE_BIN:-/usr/local/sbin/sub2api-cce-release}

install -m 0755 "$source_dir/release.py" "$destination"
printf 'installed %s\n' "$destination"
