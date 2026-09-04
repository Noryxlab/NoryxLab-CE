#!/usr/bin/env python3
"""Builds the inventory of software Noryx ships, with its licences.

A customer's compliance officer asks what runs under the platform and under
which terms. Answering from memory is how a wrong answer ends up in an audit
file, so this is derived from the actual dependency graph rather than
maintained by hand.

Three kinds of component, and they are not equally knowable:

  * Go and npm dependencies - read from the resolved graph and the licence file
    each one ships. Detected, not declared.
  * Container images - a licence cannot be derived from an image, so these are
    declared here with their upstream project, and marked as such.
  * Noryx itself - Community is MPL-2.0, Enterprise is proprietary (ADR-028).

Anything whose licence cannot be read is reported as unknown. A guess in a
compliance document is worse than a gap, because a gap gets investigated.

Run from the repository root; writes the JSON the platform serves.
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
OUTPUT = ROOT / "backend" / "internal" / "inventory" / "software-inventory.json"

# Ordered: the first marker that matches wins, so more specific texts come
# before the families they belong to.
LICENCE_MARKERS: list[tuple[str, str]] = [
    ("Mozilla Public License Version 2.0", "MPL-2.0"),
    ("Apache License", "Apache-2.0"),
    ("GNU LESSER GENERAL PUBLIC LICENSE", "LGPL"),
    ("GNU GENERAL PUBLIC LICENSE", "GPL"),
    ("Permission is hereby granted, free of charge", "MIT"),
    ("Redistribution and use in source and binary forms", "BSD"),
    ("Permission to use, copy, modify, and/or distribute this software", "ISC"),
    ("This is free and unencumbered software released into the public domain", "Unlicense"),
]

# Declared rather than detected: an image does not carry its licence, and
# inventing one from the base layer would be a guess presented as a fact.
PLATFORM_IMAGES = [
    ("Keycloak", "identity provider", "Apache-2.0", "https://github.com/keycloak/keycloak"),
    ("PostgreSQL", "database", "PostgreSQL", "https://www.postgresql.org/about/licence/"),
    ("MinIO", "object storage", "AGPL-3.0", "https://github.com/minio/minio"),
    ("Traefik", "ingress", "MIT", "https://github.com/traefik/traefik"),
    ("Longhorn", "block storage", "Apache-2.0", "https://github.com/longhorn/longhorn"),
    ("k3s", "kubernetes distribution", "Apache-2.0", "https://github.com/k3s-io/k3s"),
    ("HAProxy", "edge load balancer", "GPL-2.0-or-later", "https://www.haproxy.org/"),
    ("nginx", "frontend web server", "BSD-2-Clause", "https://nginx.org/LICENSE"),
    ("code-server", "VS Code workspace image", "MIT", "https://github.com/coder/code-server"),
    ("JupyterLab", "notebook workspace image", "BSD-3-Clause", "https://github.com/jupyterlab/jupyterlab"),
]


def detect_licence(directory: Path) -> tuple[str, str]:
    """Returns (identifier, file name) read from a module's own licence file."""
    if not directory.is_dir():
        return "unknown", ""
    for name in sorted(os.listdir(directory)):
        if not name.upper().startswith(("LICENSE", "LICENCE", "COPYING")):
            continue
        try:
            text = (directory / name).read_text(encoding="utf-8", errors="replace")
        except OSError:
            continue
        for marker, identifier in LICENCE_MARKERS:
            if marker.lower() in text.lower():
                return identifier, name
        return "unknown", name
    return "unknown", ""


def go_dependencies() -> list[dict]:
    # Download first: a module the toolchain has not fetched has no local
    # directory, so its licence would be reported unknown for want of looking
    # rather than for want of a licence.
    subprocess.run(["go", "mod", "download", "all"], cwd=ROOT / "backend",
                   capture_output=True, text=True, check=False)
    raw = subprocess.run(
        ["go", "list", "-m", "-json", "all"],
        cwd=ROOT / "backend", capture_output=True, text=True, check=True,
    ).stdout

    decoder, index, modules = json.JSONDecoder(), 0, []
    while index < len(raw):
        while index < len(raw) and raw[index] in " \n\t":
            index += 1
        if index >= len(raw):
            break
        module, index = decoder.raw_decode(raw, index)
        modules.append(module)

    out = []
    for module in modules:
        if module.get("Main"):
            continue
        identifier, source = detect_licence(Path(module["Dir"])) if module.get("Dir") else ("unknown", "")
        out.append({
            "name": module["Path"],
            "version": module.get("Version", ""),
            "licence": identifier,
            "component": "backend",
            "origin": "detected" if source else "unresolved",
            "licenceFile": source,
        })
    return out


def npm_dependencies() -> list[dict]:
    manifest = json.loads((ROOT / "frontend" / "package.json").read_text())
    modules = ROOT / "frontend" / "node_modules"
    out = []
    for section, runtime in (("dependencies", True), ("devDependencies", False)):
        for name in sorted(manifest.get(section, {})):
            package = modules / name / "package.json"
            licence, version, origin = "unknown", "", "unresolved"
            if package.is_file():
                data = json.loads(package.read_text())
                version = data.get("version", "")
                raw = data.get("license") or data.get("licenses")
                if isinstance(raw, list) and raw:
                    raw = raw[0].get("type") if isinstance(raw[0], dict) else raw[0]
                if isinstance(raw, dict):
                    raw = raw.get("type")
                if isinstance(raw, str) and raw.strip():
                    licence, origin = raw.strip(), "declared"
            out.append({
                "name": name,
                "version": version,
                "licence": licence,
                # Build-time tooling never reaches a customer's platform, and
                # saying so avoids a compliance review of software that is not
                # deployed.
                "component": "frontend" if runtime else "build tooling",
                "origin": origin,
                "licenceFile": "",
            })
    return out


def noryx_components() -> list[dict]:
    return [
        {"name": "NoryxLab Community Edition", "version": "", "licence": "MPL-2.0",
         "component": "platform", "origin": "declared", "licenceFile": "LICENSE"},
        {"name": "NoryxLab Enterprise Edition", "version": "", "licence": "Proprietary",
         "component": "platform", "origin": "declared", "licenceFile": ""},
    ]


def platform_images() -> list[dict]:
    return [{
        "name": name, "version": "", "licence": licence,
        "component": "infrastructure", "origin": "declared",
        "licenceFile": "", "role": role, "upstream": upstream,
    } for name, role, licence, upstream in PLATFORM_IMAGES]


def main() -> int:
    items = noryx_components() + go_dependencies() + npm_dependencies() + platform_images()
    items.sort(key=lambda item: (item["component"], item["name"].lower()))

    unknown = [item["name"] for item in items if item["licence"] == "unknown"]
    inventory = {
        "generatedAt": datetime.now(timezone.utc).isoformat(timespec="seconds"),
        "note": "Dependencies are read from the resolved graph; infrastructure "
                "licences are declared from their upstream project. Anything "
                "unreadable is reported as unknown rather than guessed.",
        "counts": {
            "total": len(items),
            "unknown": len(unknown),
        },
        "items": items,
    }

    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    OUTPUT.write_text(json.dumps(inventory, indent=2, ensure_ascii=False) + "\n")

    print(f"{len(items)} components written to {OUTPUT.relative_to(ROOT)}")
    if unknown:
        print(f"{len(unknown)} licence(s) could not be read: {', '.join(sorted(unknown))}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
