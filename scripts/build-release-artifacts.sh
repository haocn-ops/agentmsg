#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
output_dir="${1:-$repo_root/dist/release}"

bash "$repo_root/scripts/check-release-artifacts.sh"

rm -rf "$output_dir"
mkdir -p "$output_dir"

(
  cd "$repo_root/sdk/nodejs"
  export npm_config_cache="${TMPDIR:-/tmp}/agentmsg-npm-cache"
  export npm_config_loglevel=error
  package_name="$(
    npm pack --json | node -e "const fs = require('fs'); const payload = JSON.parse(fs.readFileSync(0, 'utf8')); process.stdout.write(payload[0].filename);"
  )"
  mv "$package_name" "$output_dir/"
)

(
  cd "$repo_root/sdk/python"
  rm -rf build dist ./*.egg-info
  python3 setup.py -q sdist --dist-dir "$output_dir" >/dev/null
  rm -rf build dist ./*.egg-info
  python3 setup.py -q bdist_wheel --dist-dir "$output_dir" >/dev/null
)

tar -C "$repo_root/sdk/go" -czf "$output_dir/agentmsg-go-sdk-$(tr -d '\n' < "$repo_root/VERSION").tar.gz" agentmsg

echo "release artifacts built in $output_dir"
