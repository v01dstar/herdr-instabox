#!/usr/bin/env python3
"""Dependency-free Markdown hygiene and repository-local link/anchor checks."""

from pathlib import Path
import re
import sys
from urllib.parse import unquote, urlsplit

ROOT = Path(__file__).resolve().parents[1]


def prose(text):
    """Exclude fenced examples so sample links/headings are not treated as docs."""
    lines = []
    fence = None
    for line in text.splitlines():
        match = re.match(r"^\s*(`{3,}|~{3,})", line)
        if match:
            marker = match.group(1)
            if fence is None:
                fence = marker
            elif marker[0] == fence[0] and len(marker) >= len(fence):
                fence = None
            lines.append("")
        else:
            lines.append(line if fence is None else "")
    return "\n".join(lines), fence


def anchors(text):
    found = set()
    counts = {}
    for heading in re.findall(r"^#{1,6}\s+(.+?)\s*#*\s*$", prose(text)[0], re.M):
        slug = re.sub(r"[^\w\- ]", "", heading.lower()).replace(" ", "-")
        count = counts.get(slug, 0)
        counts[slug] = count + 1
        found.add(slug if count == 0 else f"{slug}-{count}")
    return found


def main():
    errors = []
    files = sorted(p for p in ROOT.rglob("*.md") if ".git" not in p.relative_to(ROOT).parts)
    for path in files:
        text = path.read_text()
        relative = path.relative_to(ROOT)
        body, fence = prose(text)
        if fence:
            errors.append(f"{relative}: unclosed code fence")
        if not text.startswith("# "):
            errors.append(f"{relative}: missing top-level title")
        for number, line in enumerate(text.splitlines(), 1):
            if line.rstrip() != line:
                errors.append(f"{relative}:{number}: trailing whitespace")
        for target in re.findall(r"!?\[[^\]]*\]\(([^\s)]+)(?:\s+\"[^\"]*\")?\)", body):
            url = urlsplit(target.strip("<>"))
            if url.scheme or url.netloc:
                continue  # External reachability is intentionally not a CI network dependency.
            dest = (path.parent / unquote(url.path)).resolve() if url.path else path
            if not dest.is_relative_to(ROOT):
                errors.append(f"{relative}: link escapes repository: {target}")
            elif not dest.exists():
                errors.append(f"{relative}: missing target: {target}")
            elif url.fragment and dest.suffix == ".md" and unquote(url.fragment) not in anchors(dest.read_text()):
                errors.append(f"{relative}: missing anchor: {target}")
    if errors:
        print("\n".join(errors), file=sys.stderr)
        return 1
    print(f"Checked {len(files)} Markdown files: local links, anchors, titles, fences, whitespace.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
