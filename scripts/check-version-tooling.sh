#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
current_version="$(tr -d '\n' < "$repo_root/VERSION")"

if ! [[ "$current_version" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
  echo "version tooling check only supports stable x.y.z versions, got: $current_version" >&2
  exit 1
fi

next_patch="${BASH_REMATCH[1]}.${BASH_REMATCH[2]}.$((BASH_REMATCH[3] + 1))"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

rsync -a --exclude .git "$repo_root/" "$tmp_dir/repo/"

(
  cd "$tmp_dir/repo"
  git init -q
  git config user.name "AgentMsg CI"
  git config user.email "ci@example.invalid"
  git add .
  git commit -q -m "baseline"
  chmod +x ./scripts/bump-version.sh ./scripts/check-release-artifacts.sh ./scripts/update-changelog.sh
  bash ./scripts/bump-version.sh "$next_patch" >/dev/null
  python3 ./scripts/generate-release-notes.py >/tmp/agentmsg-release-notes-check.md
  bash ./scripts/update-changelog.sh
)

echo "version tooling verified against next patch version $next_patch"
