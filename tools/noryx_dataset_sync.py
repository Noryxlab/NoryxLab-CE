#!/usr/bin/env python3
"""Resumable direct-to-S3 dataset importer for NoryxLab.

The control plane receives only metadata and audit confirmations. File bytes
are uploaded by curl directly to the short-lived S3 URL issued by Noryx.
"""

import argparse
import concurrent.futures
import hashlib
import json
import os
import pathlib
import subprocess
import sys
import threading
import urllib.error
import urllib.request


# What one authorisation call covers. The server refuses more than this.
BATCH = 200


def api(base, token, method, path, payload):
    request = urllib.request.Request(
        base.rstrip("/") + path,
        data=json.dumps(payload).encode(),
        method=method,
        headers={"Authorization": "Bearer " + token, "Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(request, timeout=45) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        detail = error.read().decode(errors="replace")
        raise RuntimeError(f"Noryx {method} {path}: HTTP {error.code}: {detail}") from error


def digest(path):
    checksum = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(8 * 1024 * 1024), b""):
            checksum.update(chunk)
    return checksum.hexdigest()


def batched(items, size):
    for start in range(0, len(items), size):
        yield items[start : start + size]


def object_files(source):
    return [path for path in sorted(source.rglob("*")) if path.is_file()]


def load_state(path):
    if not path.exists():
        return {}
    with path.open() as handle:
        data = json.load(handle)
    return data.get("completed", {})


def save_state(path, completed, lock):
    with lock:
        temporary = path.with_suffix(path.suffix + ".tmp")
        temporary.write_text(json.dumps({"completed": completed}, indent=2, sort_keys=True) + "\n")
        temporary.replace(path)


def authorize(args, entries):
    """Ask for one batch of upload URLs.

    One call per batch rather than one per file. A study arrives as thousands
    of small objects - SELENA is 3,993 of them - so authorising each on its
    own meant around 8,000 round trips to move one directory, and an audit
    entry for every single file.
    """
    answer = api(
        args.url,
        args.token,
        "POST",
        f"/api/v1/datasets/{args.dataset}/upload-urls",
        {"objects": [{"path": relative, "size": size} for _, relative, size, _ in entries]},
    )
    for refusal in answer.get("refused") or []:
        print(f"REFUSED {refusal.get('path')}: {refusal.get('reason')}", file=sys.stderr)
    return {item["path"]: item["url"] for item in answer.get("objects") or []}


def send(args, local, relative, size, mtime, url):
    """Transfer one object, then confirm it.

    The checksum is computed here rather than in the planning pass, so a
    resume pays for the files it actually sends and nothing else.
    """
    checksum = digest(local)
    subprocess.run(
        ["curl", "--fail", "--silent", "--show-error", "--retry", "3", "--retry-all-errors",
         "--upload-file", str(local), url],
        check=True,
    )
    api(args.url, args.token, "POST", f"/api/v1/datasets/{args.dataset}/upload-complete", {
        "path": relative, "size": size, "sha256": checksum,
    })
    return relative, {"size": size, "mtime": mtime, "sha256": checksum}


def main():
    parser = argparse.ArgumentParser(description="Synchronize a directory into one Noryx dataset.")
    parser.add_argument("source", type=pathlib.Path, help="local directory to import")
    parser.add_argument("--url", required=True, help="Noryx public URL, e.g. https://datalab.noryxlab.ai")
    parser.add_argument("--dataset", required=True, help="Noryx dataset UUID")
    parser.add_argument("--token", default=os.getenv("NORYX_TOKEN"), help="datasets-scoped token; defaults to NORYX_TOKEN")
    parser.add_argument("--prefix", default="", help="destination prefix inside the dataset")
    parser.add_argument("--workers", type=int, default=4, help="simultaneous transfers (default: 4)")
    parser.add_argument("--state", type=pathlib.Path, default=pathlib.Path(".noryx-import-state.json"), help="resume-state file")
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args()
    if not args.token:
        parser.error("--token or NORYX_TOKEN is required")
    if not args.source.is_dir():
        parser.error("source must be a directory")
    if args.workers < 1 or args.workers > 16:
        parser.error("--workers must be between 1 and 16")

    completed, lock = load_state(args.state), threading.Lock()

    # Decide what to send from size and mtime, and hash only what is sent.
    #
    # Hashing every file first read the whole study before a single byte left
    # - nine gigabytes on SELENA, single-threaded, and again on every resume,
    # which is exactly when it is least welcome. Size and mtime say what has
    # not moved; the checksum is still computed for everything that is
    # actually transferred, and still recorded with it.
    entries, unchanged = [], 0
    for local in object_files(args.source):
        relative = "/".join(part for part in (args.prefix.strip("/"), local.relative_to(args.source).as_posix()) if part)
        stat = local.stat()
        size, mtime = stat.st_size, int(stat.st_mtime)
        if size > 5 * 1024**3:
            raise SystemExit(f"{relative}: {size} bytes exceeds the current 5 GiB direct-PUT limit")
        known = completed.get(relative)
        if known and known.get("size") == size and known.get("mtime") == mtime:
            unchanged += 1
            continue
        entries.append((local, relative, size, mtime))

    total = sum(entry[2] for entry in entries)
    print(f"{len(entries)} object(s) to transfer, {total / 1024**3:.1f} GiB; {unchanged} unchanged")
    if args.dry_run:
        for _, relative, size, _ in entries[:20]:
            print(f"  would send {relative} ({size} bytes)")
        if len(entries) > 20:
            print(f"  … and {len(entries) - 20} more")
        return

    failed = []
    for lot in batched(entries, BATCH):
        try:
            urls = authorize(args, [(l, r, s, m) for l, r, s, m in lot])
        except Exception as error:
            print(f"FAILED to authorize a batch of {len(lot)}: {error}", file=sys.stderr)
            failed.extend(relative for _, relative, _, _ in lot)
            continue
        with concurrent.futures.ThreadPoolExecutor(max_workers=args.workers) as pool:
            futures = {}
            for local, relative, size, mtime in lot:
                url = urls.get(relative)
                if not url:
                    failed.append(relative)
                    continue
                futures[pool.submit(send, args, local, relative, size, mtime, url)] = relative
            for future in concurrent.futures.as_completed(futures):
                relative = futures[future]
                try:
                    key, result = future.result()
                    completed[key] = result
                    save_state(args.state, completed, lock)
                    print(f"confirmed {key}")
                except Exception as error:
                    failed.append(relative)
                    print(f"FAILED {relative}: {error}", file=sys.stderr)

    if failed:
        raise SystemExit(f"{len(failed)} object(s) failed; rerun the same command to resume")
    print(f"{len(entries)} object(s) transferred and confirmed")


if __name__ == "__main__":
    main()
