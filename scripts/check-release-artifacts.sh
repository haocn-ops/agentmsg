#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
expected_version="$(tr -d '\n' < "$repo_root/VERSION")"

node_package_version="$(node -p "require('$repo_root/sdk/nodejs/package.json').version")"
node_lock_version="$(node -p "require('$repo_root/sdk/nodejs/package-lock.json').version")"
node_source_version="$(sed -n "s/^export const VERSION = '\\(.*\\)';$/\\1/p" "$repo_root/sdk/nodejs/agentmsg/version.ts")"
python_version="$(python3 -c "import pathlib,re; data=pathlib.Path('$repo_root/sdk/python/agentmsg/__init__.py').read_text(); print(re.search(r'^__version__ = \"([^\"]+)\"', data, re.M).group(1))")"
pyproject_version="$(python3 -c "import pathlib,re; data=pathlib.Path('$repo_root/sdk/python/pyproject.toml').read_text(); print(re.search(r'^version = \"([^\"]+)\"', data, re.M).group(1))")"

for value in \
  "$node_package_version" \
  "$node_lock_version" \
  "$node_source_version" \
  "$python_version" \
  "$pyproject_version"
do
  if [ "$value" != "$expected_version" ]; then
    echo "release version mismatch: expected $expected_version but found $value" >&2
    exit 1
  fi
done

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

(
  cd "$repo_root/sdk/nodejs"
  export npm_config_cache="$tmp_dir/npm-cache"
  export npm_config_loglevel=error
  npm pack --dry-run >/dev/null
)

(
  cd "$repo_root/sdk/python"
  python3 setup.py -q sdist --dist-dir "$tmp_dir/python-dist" >/dev/null
  PIP_CACHE_DIR="$tmp_dir/pip-cache" python3 -m pip --disable-pip-version-check wheel --no-build-isolation --no-deps --wheel-dir "$tmp_dir/python-dist" . >/dev/null
)

echo "release artifacts verified for version $expected_version"
