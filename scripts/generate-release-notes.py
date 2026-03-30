#!/usr/bin/env python3

from __future__ import annotations

import pathlib
import subprocess
import sys


REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent
VERSION = (REPO_ROOT / "VERSION").read_text(encoding="utf-8").strip()
CURRENT_TAG = f"v{VERSION}"

SECTIONS = [
    ("feat", "Features"),
    ("fix", "Fixes"),
    ("perf", "Performance"),
    ("refactor", "Refactors"),
    ("build", "Build And Packaging"),
    ("ci", "CI And Release"),
    ("release", "CI And Release"),
    ("docs", "Documentation"),
    ("test", "Testing"),
]
ORDERED_TITLES: list[str] = []
for _, title in SECTIONS:
    if title not in ORDERED_TITLES:
        ORDERED_TITLES.append(title)


def git(*args: str) -> str:
    result = subprocess.run(
        ["git", "-C", str(REPO_ROOT), *args],
        check=True,
        capture_output=True,
        text=True,
    )
    return result.stdout


def latest_previous_tag() -> str | None:
    tags = [
        line.strip()
        for line in git("tag", "--list", "v*", "--sort=-version:refname").splitlines()
        if line.strip() and line.strip() != CURRENT_TAG
    ]
    return tags[0] if tags else None


def collect_commits(commit_range: str) -> list[tuple[str, str]]:
    raw = git("log", "--no-merges", "--pretty=format:%h%x1f%s", commit_range)
    commits: list[tuple[str, str]] = []
    for line in raw.splitlines():
        if not line.strip():
            continue
        short_hash, subject = line.split("\x1f", 1)
        commits.append((short_hash, subject.strip()))
    return commits


def classify(subject: str) -> str:
    prefix = subject.split(":", 1)[0].strip().lower()
    for key, title in SECTIONS:
        if prefix == key:
            return title
    return "Other"


def grouped_commits(commits: list[tuple[str, str]]) -> dict[str, list[tuple[str, str]]]:
    groups: dict[str, list[tuple[str, str]]] = {title: [] for title in ORDERED_TITLES}
    groups["Other"] = []
    for short_hash, subject in commits:
        groups[classify(subject)].append((short_hash, subject))
    return groups


def render() -> str:
    previous_tag = latest_previous_tag()
    if previous_tag:
        commit_range = f"{previous_tag}..HEAD"
        comparison_label = f"{previous_tag} -> {CURRENT_TAG}"
    else:
        commit_range = "HEAD"
        comparison_label = f"initial release -> {CURRENT_TAG}"

    commits = collect_commits(commit_range)
    groups = grouped_commits(commits)

    lines = [
        f"# AgentMsg {CURRENT_TAG}",
        "",
        "## Scope",
        "",
        f"- Version: `{VERSION}`",
        f"- Comparison: `{comparison_label}`",
        "- Release artifacts: Node.js tarball, Python sdist, Python wheel, Go SDK source bundle",
        "",
        "## Verification",
        "",
        "- `make release-check`",
        "- `make build-release-artifacts`",
        "",
    ]

    for title in ORDERED_TITLES:
        entries = groups.get(title, [])
        if not entries:
            continue
        lines.append(f"## {title}")
        lines.append("")
        for short_hash, subject in entries:
            lines.append(f"- `{short_hash}` {subject}")
        lines.append("")

    if groups["Other"]:
        lines.append("## Other")
        lines.append("")
        for short_hash, subject in groups["Other"]:
            lines.append(f"- `{short_hash}` {subject}")
        lines.append("")

    return "\n".join(lines).rstrip() + "\n"


sys.stdout.write(render())
