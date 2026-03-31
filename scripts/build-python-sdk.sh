#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
output_dir="${1:-$repo_root/sdk/python/dist}"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

mkdir -p "$output_dir"
rsync -a \
  --exclude build \
  --exclude dist \
  --exclude '*.egg-info' \
  "$repo_root/sdk/python/" "$tmp_dir/src/"

cd "$tmp_dir/src"

if python3 -m build --version >/dev/null 2>&1; then
  python3 -m build --sdist --wheel --outdir "$output_dir" >/dev/null
else
  echo "python -m build is required to package the Python SDK" >&2
  echo "install it with: python3 -m pip install build" >&2
  exit 1
fi
