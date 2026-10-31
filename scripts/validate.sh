#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
tool_dir="${TOOL_DIR:-$PWD/.tools}"
render_dir="${RENDER_DIR:-$PWD/.rendered}"
mkdir -p "$tool_dir" "$render_dir"
sha256sum --check --strict schemas/SHA256SUMS
go build -trimpath -o "$tool_dir/validate" ./cmd/validate
status=0
for environment in prod; do
  echo "Validating $environment"
  output="$render_dir/$environment.yaml"
  if "$tool_dir/kustomize" build "overlays/$environment" > "$output"; then
    # Local standalone strict schemas only: no network or live API fallback.
    if ! "$tool_dir/kubeconform" -strict -summary -exit-on-error \
      -kubernetes-version 1.36.0 \
      -schema-location 'schemas/{{.ResourceKind}}{{.KindSuffix}}.json' "$output"; then
      status=1
    fi
    if ! "$tool_dir/validate" -environment "$environment" "$output"; then
      status=1
    fi
  else
    status=1
  fi
done
exit "$status"
