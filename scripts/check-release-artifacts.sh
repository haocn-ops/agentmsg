#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
expected_version="$(tr -d '\n' < "$repo_root/VERSION")"
expected_go_module="github.com/haocn-ops/agentmsg/sdk/go/agentmsg"

node_package_version="$(node -p "require('$repo_root/sdk/nodejs/package.json').version")"
node_lock_version="$(node -p "require('$repo_root/sdk/nodejs/package-lock.json').version")"
node_source_version="$(sed -n "s/^export const VERSION = '\\(.*\\)';$/\\1/p" "$repo_root/sdk/nodejs/agentmsg/version.ts")"
python_version="$(python3 -c "import pathlib,re; data=pathlib.Path('$repo_root/sdk/python/agentmsg/__init__.py').read_text(); print(re.search(r'^__version__ = \"([^\"]+)\"', data, re.M).group(1))")"
pyproject_version="$(python3 -c "import pathlib,re; data=pathlib.Path('$repo_root/sdk/python/pyproject.toml').read_text(); print(re.search(r'^version = \"([^\"]+)\"', data, re.M).group(1))")"
go_sdk_version="$(python3 -c "import pathlib,re; data=pathlib.Path('$repo_root/sdk/go/agentmsg/version.go').read_text(); print(re.search(r'^const Version = \"([^\"]+)\"', data, re.M).group(1))")"
go_module_path="$(sed -n 's/^module //p' "$repo_root/sdk/go/agentmsg/go.mod")"

for value in \
  "$node_package_version" \
  "$node_lock_version" \
  "$node_source_version" \
  "$python_version" \
  "$pyproject_version" \
  "$go_sdk_version"
do
  if [ "$value" != "$expected_version" ]; then
    echo "release version mismatch: expected $expected_version but found $value" >&2
    exit 1
  fi
done

if [ "$go_module_path" != "$expected_go_module" ]; then
  echo "go module path mismatch: expected $expected_go_module but found $go_module_path" >&2
  exit 1
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT
go_mod_cache="${AGENTMSG_GO_MODCACHE:-/tmp/agentmsg-go-sdk-mod}"
go_path="${AGENTMSG_GO_GOPATH:-/tmp/agentmsg-go-sdk-path}"

(
  cd "$repo_root/sdk/nodejs"
  export npm_config_cache="$tmp_dir/npm-cache"
  export npm_config_loglevel=error
  npm pack --dry-run >/dev/null
)

(
  cd "$repo_root/sdk/python"
  rm -rf build dist ./*.egg-info
  python3 setup.py -q sdist --dist-dir "$tmp_dir/python-dist" >/dev/null
  rm -rf build dist ./*.egg-info
  python3 setup.py -q bdist_wheel --dist-dir "$tmp_dir/python-dist" >/dev/null
)

(
  cd "$repo_root/sdk/go/agentmsg"
  GOCACHE="$tmp_dir/go-build" GOMODCACHE="$go_mod_cache" GOPATH="$go_path" go test ./... >/dev/null
)

echo "release artifacts verified for version $expected_version"
