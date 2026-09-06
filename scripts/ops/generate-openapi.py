#!/usr/bin/env python3
"""
Fills the OpenAPI document with the routes the platform actually serves.

The document described 40 paths while the router registered 181, and nothing
said so: a client reading it saw a fifth of the API and had no way to know. The
missing four fifths were not secret, only undocumented, which is the worse of
the two - it teaches people to read the code instead.

This does not replace hand-written entries. It reads the router, finds the
routes with no entry, and writes one for each: the method, the path parameters,
a summary derived from the handler's name, and the responses every endpoint on
this platform can return. Anything already in the document is left exactly as
it is, because a generated line is worth less than a written one and must never
overwrite it.

    python3 scripts/ops/generate-openapi.py [--check]

--check writes nothing and exits non-zero if a route is undocumented. That is
what CI runs, and it is the part that keeps this from drifting again.
"""

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
DOCUMENT = ROOT / "backend/internal/http/static/openapi.yaml"
ROUTE_FILES = [
    ROOT / "backend/internal/http/server.go",
    ROOT.parent / "NoryxLab-EE/overlay/backend/internal/http/ee_routes.go",
]

ROUTE = re.compile(r'mux\.HandleFunc\("(GET|POST|PUT|DELETE|PATCH) ([^"]+)",\s*([A-Za-z0-9_.]+)\)')

# Which family an endpoint belongs to, read from its path. Tags are how a
# reader finds anything in a document this size.
TAGS = [
    ("/api/v1/admin/backups", "Backups"),
    ("/api/v1/admin", "Admin"),
    ("/api/v1/projects", "Projects"),
    ("/api/v1/workspaces", "Workloads"),
    ("/api/v1/jobs", "Workloads"),
    ("/api/v1/builds", "Workloads"),
    ("/api/v1/apps", "Workloads"),
    ("/api/v1/dashboards", "Workloads"),
    ("/api/v1/cronjobs", "Workloads"),
    ("/api/v1/datasets", "Datasets"),
    ("/api/v1/datasources", "Datasets"),
    ("/api/v1/ontologies", "Datasets"),
    ("/api/v1/repositories", "Repositories"),
    ("/api/v1/secrets", "Secrets"),
    ("/api/v1/user", "Account"),
    ("/api/v1/auth", "Auth"),
    ("/api/v1/egress", "Governance"),
    ("/workspaces", "Proxies"),
    ("/apps", "Proxies"),
    ("/dashboards", "Proxies"),
]


def routes():
    found = {}
    for path in ROUTE_FILES:
        if not path.exists():
            continue
        for method, route, handler in ROUTE.findall(path.read_text()):
            # `{path...}` is the router's wildcard; OpenAPI has no notion of one,
            # so it is documented as an ordinary parameter.
            found[(method, route.replace("...", ""))] = handler.split(".")[-1]
    return found


def documented(text):
    """(path, method) pairs already in the document, and where each path starts."""
    pairs, starts = set(), {}
    current = None
    in_paths = False
    for number, line in enumerate(text.splitlines()):
        if line.startswith("paths:"):
            in_paths = True
            continue
        if in_paths and line and not line.startswith(" "):
            in_paths = False
        if not in_paths:
            continue
        path_match = re.match(r"^  (/\S*):\s*$", line)
        if path_match:
            current = path_match.group(1)
            starts[current] = number
            continue
        method_match = re.match(r"^    (get|post|put|delete|patch):\s*$", line)
        if method_match and current:
            pairs.add((current, method_match.group(1).upper()))
    return pairs, starts


def summarise(handler, method, path):
    words = re.sub(r"(?<!^)(?=[A-Z])", " ", handler).strip()
    return words[0].upper() + words[1:] if words else f"{method} {path}"


def tag_for(path):
    for prefix, tag in TAGS:
        if path.startswith(prefix):
            return tag
    return "Platform"


def block(method, path, handler, indent="    "):
    parameters = re.findall(r"\{(\w+)\}", path)
    lines = [f"{indent}{method.lower()}:"]
    lines.append(f"{indent}  tags: [{tag_for(path)}]")
    lines.append(f"{indent}  summary: {summarise(handler, method, path)}")
    lines.append(f"{indent}  operationId: {handler[0].lower() + handler[1:]}")
    lines.append(f"{indent}  security:")
    lines.append(f"{indent}    - bearerAuth: []")
    lines.append(f"{indent}    - xNoryxUser: []")
    if parameters:
        lines.append(f"{indent}  parameters:")
        for name in parameters:
            lines.append(f"{indent}    - name: {name}")
            lines.append(f"{indent}      in: path")
            lines.append(f"{indent}      required: true")
            lines.append(f"{indent}      schema: {{ type: string }}")
    lines.append(f"{indent}  responses:")
    lines.append(f"{indent}    '200':")
    lines.append(f"{indent}      description: Success")
    for code, ref in (("401", "Unauthorized"), ("403", "Forbidden"), ("404", "NotFound")):
        lines.append(f"{indent}    '{code}':")
        lines.append(f"{indent}      $ref: '#/components/responses/{ref}'")
    return lines


def main():
    check_only = "--check" in sys.argv
    text = DOCUMENT.read_text()
    pairs, starts = documented(text)
    registered = routes()

    missing = sorted(
        (path, method, handler)
        for (method, path), handler in registered.items()
        if (path, method) not in pairs
    )
    if not missing:
        print(f"  every one of the {len(registered)} registered routes is documented")
        return 0
    if check_only:
        print(f"  {len(missing)} route(s) the router serves and the document does not describe:")
        for path, method, _ in missing[:20]:
            print(f"    {method} {path}")
        if len(missing) > 20:
            print(f"    ... and {len(missing) - 20} more")
        print("  run scripts/ops/generate-openapi.py to add them, then write real summaries.")
        return 1

    lines = text.splitlines()
    # Group by path so a new path is written once with all of its methods.
    by_path = {}
    for path, method, handler in missing:
        by_path.setdefault(path, []).append((method, handler))

    additions = []
    for path in sorted(by_path):
        if path in starts:
            continue  # handled below, inserted into the existing block
        additions.append(f"  {path}:")
        for method, handler in sorted(by_path[path]):
            additions.extend(block(method, path, handler))

    # Insert methods into paths that already exist, from the bottom up so the
    # line numbers above stay valid.
    for path in sorted(by_path, key=lambda p: -starts.get(p, -1)):
        if path not in starts:
            continue
        insert_at = starts[path] + 1
        payload = []
        for method, handler in sorted(by_path[path]):
            payload.extend(block(method, path, handler))
        lines[insert_at:insert_at] = payload

    end = len(lines)
    for number in range(len(lines) - 1, -1, -1):
        if lines[number].strip() and not lines[number].startswith(" "):
            if not lines[number].startswith("paths:"):
                end = number
            break
    lines[end:end] = additions

    DOCUMENT.write_text("\n".join(lines) + "\n")
    print(f"  documented {len(missing)} route(s); write real summaries for the ones that matter")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
