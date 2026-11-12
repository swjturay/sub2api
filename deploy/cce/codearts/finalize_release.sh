#!/bin/sh
set -eu

build_number=${1:?build number is required}
. deploy/cce/codearts/build-metadata.env
test -n "$SOURCE_SHA"
test -n "$SOURCE_VERSION"

repository=registry.cn-shanghai.aliyuncs.com/rayless/sub2api
backend_tag="$repository:backend-codearts-$build_number"
insights_tag="$repository:insights-codearts-$build_number"

backend_image=$(python3 deploy/cce/codearts/registry_digest.py --image "$backend_tag")
insights_image=$(python3 deploy/cce/codearts/registry_digest.py --image "$insights_tag")
output=release
python3 deploy/cce/codearts/render_release.py \
  --source-sha "$SOURCE_SHA" \
  --version "$SOURCE_VERSION" \
  --pipeline-run "$build_number" \
  --backend-image "$backend_image" \
  --insights-image "$insights_image" \
  --output-dir "$output"

python3 -m json.tool "$output/cce-release-manifest.json" >/dev/null
python3 deploy/cce/build_bundle.py \
  --release-dir "$output" \
  --release-tool deploy/cce/release.py \
  --output-dir "$output" \
  --archive-name sub2api-release-bundle.tar.gz
sha256sum -c "$output/sub2api-release-bundle.tar.gz.sha256"
printf 'Rendered immutable operations-host release bundle for %s\n' "$SOURCE_SHA"
