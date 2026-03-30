#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "usage: $0 <new-version>" >&2
  exit 1
fi

new_version="$1"

if ! [[ "$new_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$ ]]; then
  echo "invalid semantic version: $new_version" >&2
  exit 1
fi

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
current_version="$(tr -d '\n' < "$repo_root/VERSION")"

if [ "$new_version" = "$current_version" ]; then
  echo "version is already $new_version"
  exit 0
fi

printf '%s\n' "$new_version" > "$repo_root/VERSION"

TARGET_VERSION="$new_version" REPO_ROOT="$repo_root" node <<'EOF'
const fs = require('fs');
const path = require('path');

const repoRoot = process.env.REPO_ROOT;
const nextVersion = process.env.TARGET_VERSION;

function writeJson(filePath) {
  const absolutePath = path.join(repoRoot, filePath);
  const data = JSON.parse(fs.readFileSync(absolutePath, 'utf8'));
  data.version = nextVersion;
  if (data.packages && data.packages['']) {
    data.packages[''].version = nextVersion;
  }
  fs.writeFileSync(absolutePath, JSON.stringify(data, null, 2) + '\n');
}

writeJson('sdk/nodejs/package.json');
writeJson('sdk/nodejs/package-lock.json');
EOF

perl -0pi -e "s/export const VERSION = '[^']+';/export const VERSION = '$new_version';/" \
  "$repo_root/sdk/nodejs/agentmsg/version.ts"
perl -0pi -e "s/__version__ = \"[^\"]+\"/__version__ = \"$new_version\"/" \
  "$repo_root/sdk/python/agentmsg/__init__.py"
perl -0pi -e "s/^version = \"[^\"]+\"/version = \"$new_version\"/m" \
  "$repo_root/sdk/python/pyproject.toml"
perl -0pi -e "s/^const Version = \"[^\"]+\"/const Version = \"$new_version\"/m" \
  "$repo_root/sdk/go/agentmsg/version.go"

bash "$repo_root/scripts/check-release-artifacts.sh"

echo "bumped version: $current_version -> $new_version"
