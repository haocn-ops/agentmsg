#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
changelog_path="$repo_root/CHANGELOG.md"
tmp_file="$(mktemp)"
trap 'rm -f "$tmp_file"' EXIT

python3 "$repo_root/scripts/generate-release-notes.py" > "$tmp_file"

if [ ! -f "$changelog_path" ]; then
  cat > "$changelog_path" <<'EOF'
# Changelog

All notable changes to this project will be documented in this file.

EOF
fi

python3 - <<'PY' "$changelog_path" "$tmp_file"
from __future__ import annotations

import pathlib
import re
import sys

changelog_path = pathlib.Path(sys.argv[1])
notes_path = pathlib.Path(sys.argv[2])

existing = changelog_path.read_text(encoding="utf-8")
notes = notes_path.read_text(encoding="utf-8").strip()
header = "# Changelog\n\nAll notable changes to this project will be documented in this file.\n"
release_heading = notes.splitlines()[0]

body = existing
if body.startswith(header):
    body = body[len(header):].lstrip("\n")
else:
    body = body.strip()

pattern = re.compile(
    rf"(?ms)^{re.escape(release_heading)}\n.*?(?=^# AgentMsg |\Z)"
)
body = pattern.sub("", body).strip()

updated = header + "\n" + notes + "\n\n"
if body:
    updated += body.strip() + "\n"

changelog_path.write_text(updated, encoding="utf-8")
PY
