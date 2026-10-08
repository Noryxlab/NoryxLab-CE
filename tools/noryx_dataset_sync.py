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
import urllib.parse
import urllib.request


# What one authorisation call covers. The server refuses more than this.
BATCH = 200


class DirectUploadUnavailable(RuntimeError):
    """The dataset's object store is not reachable from outside the cluster."""


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
        if error.code == 409 and '"direct_upload_unavailable"' in detail:
            raise DirectUploadUnavailable(detail) from error
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
    """Read the resume journal: one JSON record per confirmed object.

    Still reads the old single-object format, so an import interrupted under
    the previous version resumes instead of starting over.
    """
    if not path.exists():
        return {}
    text = path.read_text()
    if text.lstrip().startswith("{") and '"completed"' in text.split("\n", 1)[0]:
        try:
            return json.loads(text).get("completed", {})
        except json.JSONDecodeError:
            pass
    completed = {}
    for line in text.splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            record = json.loads(line)
        # A journal truncated by a kill keeps every whole line before the
        # broken one: resuming from most of it beats starting over.
        except json.JSONDecodeError:
            continue
        completed[record.pop("path")] = record
    return completed


def record_state(path, relative, result, lock):
    """Append one line, instead of rewriting everything.

    The previous version serialised the whole map after every confirmed
    object, holding a lock while it did. That is quadratic in bytes written
    and it is the wall this hits first: at 435,000 objects - a terabyte of
    2.3 MB DICOM slices - the map is about 48 MB, so moving one terabyte of
    data would write ten terabytes of state, with every worker queued behind
    each serialisation. On a study of four thousand files it is merely
    wasteful, which is why it went unnoticed.
    """
    with lock:
        with path.open("a") as handle:
            handle.write(json.dumps({"path": relative, **result}, sort_keys=True) + "\n")


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


def send_through_noryx(args, local, relative, size, mtime):
    """Send one object through the API, for a store that is not public.

    The platform's internal object store answers on a cluster address, so a
    presigned URL for it resolves nowhere outside. The bytes go through the
    API instead: slower, and the only thing that works. No confirmation call
    - the platform wrote the object itself, so it already knows.
    """
    checksum = digest(local)
    with local.open("rb") as handle:
        request = urllib.request.Request(
            args.url.rstrip("/") + f"/api/v1/datasets/{args.dataset}/objects/" + urllib.parse.quote(relative),
            data=handle.read(),
            method="PUT",
            headers={
                "Authorization": "Bearer " + args.token,
                "Content-Type": "application/octet-stream",
            },
        )
        try:
            urllib.request.urlopen(request, timeout=300).read()
        except urllib.error.HTTPError as error:
            detail = error.read().decode(errors="replace")
            raise RuntimeError(f"PUT {relative}: HTTP {error.code}: {detail}") from error
    return relative, {"size": size, "mtime": mtime, "sha256": checksum}


def send(args, local, relative, size, mtime, url):
    """Transfer one object. Confirmation happens for the batch, afterwards.

    The checksum is computed here rather than in the planning pass, so a
    resume pays for the files it actually sends and nothing else.
    """
    checksum = digest(local)
    subprocess.run(
        ["curl", "--fail", "--silent", "--show-error", "--retry", "3", "--retry-all-errors",
         "--upload-file", str(local), url],
        check=True,
    )
    return relative, {"size": size, "mtime": mtime, "sha256": checksum}


def confirm(args, sent):
    """Confirm one batch, and say which objects the platform could not verify.

    Per-object confirmation was the whole remaining cost once authorisation
    was batched: a terabyte of DICOM slices is some 435,000 objects, so one
    call each is 435,000 round trips to close an import whose URLs took two
    thousand.
    """
    answer = api(
        args.url,
        args.token,
        "POST",
        f"/api/v1/datasets/{args.dataset}/upload-completions",
        {"objects": [
            {"path": relative, "size": result["size"], "sha256": result["sha256"]}
            for relative, result in sent
        ]},
    )
    refused = set()
    for failure in answer.get("failed") or []:
        refused.add(failure.get("path"))
        print(f"UNVERIFIED {failure.get('path')}: {failure.get('reason')}", file=sys.stderr)
    return refused


def main():
    parser = argparse.ArgumentParser(description="Synchronize a directory into one Noryx dataset.")
    parser.add_argument("source", type=pathlib.Path, help="local directory to import")
    parser.add_argument("--url", required=True, help="Noryx public URL, e.g. https://datalab.noryxlab.ai")
    parser.add_argument("--dataset", required=True, help="Noryx dataset UUID")
    parser.add_argument("--token", default=os.getenv("NORYX_TOKEN"), help="datasets-scoped token; defaults to NORYX_TOKEN")
    parser.add_argument("--prefix", default="", help="destination prefix inside the dataset")
    parser.add_argument("--workers", type=int, default=4, help="simultaneous transfers (default: 4)")
    parser.add_argument("--state", type=pathlib.Path, default=pathlib.Path(".noryx-import-state.jsonl"), help="resume journal")
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

    failed, through_api = [], False
    for lot in batched(entries, BATCH):
        urls = {}
        if not through_api:
            try:
                urls = authorize(args, [(l, r, s, m) for l, r, s, m in lot])
            except DirectUploadUnavailable:
                # Said once, then simply taken: repeating it per batch would
                # bury the hundred lines that matter.
                print("direct upload is not available for this dataset; "
                      "sending through the API instead", file=sys.stderr)
                through_api = True
            except Exception as error:
                print(f"FAILED to authorize a batch of {len(lot)}: {error}", file=sys.stderr)
                failed.extend(relative for _, relative, _, _ in lot)
                continue
        sent = []
        with concurrent.futures.ThreadPoolExecutor(max_workers=args.workers) as pool:
            futures = {}
            for local, relative, size, mtime in lot:
                if through_api:
                    futures[pool.submit(send_through_noryx, args, local, relative, size, mtime)] = relative
                    continue
                url = urls.get(relative)
                if not url:
                    failed.append(relative)
                    continue
                futures[pool.submit(send, args, local, relative, size, mtime, url)] = relative
            for future in concurrent.futures.as_completed(futures):
                relative = futures[future]
                try:
                    sent.append(future.result())
                except Exception as error:
                    failed.append(relative)
                    print(f"FAILED {relative}: {error}", file=sys.stderr)

        if not sent:
            continue
        # Nothing to confirm when the platform wrote the object itself.
        if through_api:
            for relative, result in sent:
                completed[relative] = result
                record_state(args.state, relative, result, lock)
            print(f"{len(sent)} sent ({len(completed)} total)")
            continue
        try:
            refused = confirm(args, sent)
        except Exception as error:
            print(f"FAILED to confirm a batch of {len(sent)}: {error}", file=sys.stderr)
            failed.extend(relative for relative, _ in sent)
            continue
        # Only what the platform verified goes into the journal: an object
        # recorded as done that is not actually there would be skipped by
        # every later resume, which is the one failure a resume must not have.
        for relative, result in sent:
            if relative in refused:
                failed.append(relative)
                continue
            completed[relative] = result
            record_state(args.state, relative, result, lock)
        print(f"{len(sent) - len(refused)} confirmed ({len(completed)} total)")

    if failed:
        raise SystemExit(f"{len(failed)} object(s) failed; rerun the same command to resume")
    print(f"{len(entries)} object(s) transferred and confirmed")


if __name__ == "__main__":
    main()
