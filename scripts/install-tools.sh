#!/usr/bin/env bash
set -euo pipefail
# Exact reviewed Linux/amd64 tools; checksums are from the upstream releases.
tool_dir="${TOOL_DIR:-$PWD/.tools}"
mkdir -p "$tool_dir"
download_dir=$(mktemp -d)
trap 'rm -rf -- "$download_dir"' EXIT
curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 \
  'https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv5.8.1/kustomize_v5.8.1_linux_amd64.tar.gz' \
  -o "$download_dir/kustomize.tar.gz"
curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 \
  'https://github.com/yannh/kubeconform/releases/download/v0.8.0/kubeconform-linux-amd64.tar.gz' \
  -o "$download_dir/kubeconform.tar.gz"
(
  cd "$download_dir"
  printf '%s  %s\n' \
    '029a7f0f4e1932c52a0476cf02a0fd855c0bb85694b82c338fc648dcb53a819d' 'kustomize.tar.gz' \
    '9bc2bffbf71f261128533edaf912153948b7ff238f9a531ae6d34466ec287883' 'kubeconform.tar.gz' | sha256sum --check --strict -
)
tar -xzf "$download_dir/kustomize.tar.gz" -C "$tool_dir" kustomize
tar -xzf "$download_dir/kubeconform.tar.gz" -C "$tool_dir" kubeconform
"$tool_dir/kustomize" version
"$tool_dir/kubeconform" -v
