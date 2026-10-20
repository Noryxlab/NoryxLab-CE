#!/usr/bin/env python3
"""
The platform must refresh the images it starts.

Two manifests name the environment images: `noryx-api.yaml`, which decides
what a workspace starts from, and `noryx-environment-rebuild.yaml`, which
decides what the nightly rebuild refreshes. They are written by hand, in two
places, and they drifted: the deployed platform moved VS Code to 0.1.3 while
the CronJob went on rebuilding 0.1.2 every night for weeks. Nothing failed.
The build succeeded, pushed a fresh image, and nobody started it - so a
security fix in the base image reached the registry and never reached a user.

That is the worst shape a defect can take: a job that works perfectly on the
wrong target, reporting success. This refuses it.

    python3 scripts/ops/check-environment-image-tags.py [--check]

Without --check it prints what it found. With --check it exits non-zero on a
disagreement, which is what CI runs.
"""
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
API = ROOT / "deploy/k8s/base/noryx-api.yaml"
REBUILD = ROOT / "deploy/k8s/base/noryx-environment-rebuild.yaml"

REFERENCE = re.compile(r"[\w.\-]+/noryx-environments/(noryx-[\w\-]+):([\w.\-]+)")


def references(path: Path, keep) -> dict[str, str]:
    found: dict[str, str] = {}
    for line in path.read_text().splitlines():
        if not keep(line):
            continue
        match = REFERENCE.search(line)
        if match:
            found[match.group(1)] = match.group(2)
    return found


def main() -> int:
    # Only the lines that carry a decision: a comment naming an old tag is
    # documentation, not configuration, and failing on one would teach people
    # to delete comments.
    started = references(API, lambda l: "value:" in l and "_IMAGE" not in l)
    refreshed = references(REBUILD, lambda l: "--destination=" in l)

    problems: list[str] = []
    for name, tag in sorted(refreshed.items()):
        if name not in started:
            problems.append(
                f"{name}: the nightly rebuild refreshes {tag}, which this "
                f"platform never starts")
        elif started[name] != tag:
            problems.append(
                f"{name}: started as {started[name]}, rebuilt as {tag} - the "
                f"rebuild refreshes an image nobody runs")

    uncovered = sorted(set(started) - set(refreshed))

    if "--check" not in sys.argv:
        for name in sorted(started):
            mark = "refreshed" if name in refreshed else "not refreshed here"
            print(f"  {name:22} {started[name]:12} {mark}")
    if uncovered:
        print("  note: no nightly rebuild for " + ", ".join(uncovered) +
              " (built by another pipeline, or ageing unattended)")
    if problems:
        for problem in problems:
            print("  " + problem, file=sys.stderr)
        return 1
    if "--check" in sys.argv:
        print(f"  the {len(refreshed)} rebuilt environment image(s) match what the platform starts")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
