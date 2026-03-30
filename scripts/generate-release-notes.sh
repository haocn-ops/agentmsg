#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
current_version="$(tr -d '\n' < "$repo_root/VERSION")"
current_tag="v$current_version"

previous_tag="$(
  git -C "$repo_root" tag --list 'v*' --sort=-version:refname | grep -vx "$current_tag" | head -n 1 || true
)"

if [ -n "$previous_tag" ]; then
  commit_range="$previous_tag..HEAD"
  comparison_label="$previous_tag -> $current_tag"
else
  commit_range="HEAD"
  comparison_label="initial release -> $current_tag"
fi

cat <<EOF
# AgentMsg $current_tag

## Scope

- Version: \`$current_version\`
- Comparison: \`$comparison_label\`
- Release artifacts: Node.js tarball, Python sdist, Python wheel, Go SDK source bundle

## Verification

- \`make release-check\`
- \`make build-release-artifacts\`

## Commits
EOF

git -C "$repo_root" log --no-merges --pretty=format:'- %h %s' "$commit_range"
echo
