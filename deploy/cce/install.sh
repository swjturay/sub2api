#!/bin/sh
set -eu

source_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
release_destination=${SUB2API_CCE_RELEASE_BIN:-/usr/local/sbin/sub2api-cce-release}
host_destination=${SUB2API_CCE_HOST_BIN:-/usr/local/sbin/sub2api-cce-host}

install -m 0755 "$source_dir/release.py" "$release_destination"
install -m 0755 "$source_dir/host_release.py" "$host_destination"
printf 'installed %s\ninstalled %s\n' "$release_destination" "$host_destination"
