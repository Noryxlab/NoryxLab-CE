#!/usr/bin/env python3
"""Inject each incident of the corpus, capture the evidence, tear it down.

The capture is the point. A support agent is only worth measuring against
incidents whose cause is known in advance, and a platform that has not failed
much in production has to create its failures on purpose. Each capture is the
exact evidence the platform hands a reader - the pod's phase and the events
Kubernetes recorded - so the fixtures score the whole chain rather than a
model's prose.

Every incident lives in its own namespace and is deleted afterwards. Nothing
touches the platform's namespaces.

    scripts/ops/capture-incidents.py --context noryx-test
    scripts/ops/capture-incidents.py --context noryx-test --only image-unavailable
"""

from __future__ import annotations

import argparse
import json
import pathlib
import subprocess
import sys
import time

import yaml

HERE = pathlib.Path(__file__).resolve().parent
CORPUS = HERE / "incidents" / "corpus.yaml"
DEFAULT_OUT = HERE.parents[1] / "backend" / "internal" / "http" / "handlers" / "testdata" / "incidents"


def kubectl(context: str, *args: str, check: bool = True, stdin: str | None = None) -> str:
    command = ["kubectl", "--context", context, *args]
    result = subprocess.run(command, capture_output=True, text=True, input=stdin)
    if check and result.returncode != 0:
        raise SystemExit(f"{' '.join(command)}\n{result.stderr.strip()}")
    return result.stdout


def pod_manifest(incident: dict, namespace: str) -> str:
    spec = incident["pod"]
    name = incident["id"]
    container: dict = {
        "name": "workload",
        "image": spec["image"],
        "command": spec["command"],
    }
    if "resources" in spec:
        container["resources"] = spec["resources"]

    pod: dict = {
        "apiVersion": "v1",
        "kind": "Pod",
        "metadata": {"name": name, "namespace": namespace, "labels": {"noryx-incident": name}},
        "spec": {
            "restartPolicy": spec.get("restartPolicy", "Never"),
            "containers": [container],
            # A failing pod must not be retried onto another node for minutes:
            # the corpus wants the first, honest failure.
            "terminationGracePeriodSeconds": 0,
        },
    }

    documents = []
    if "pvc" in spec:
        claim = spec["pvc"]
        documents.append({
            "apiVersion": "v1",
            "kind": "PersistentVolumeClaim",
            "metadata": {"name": name, "namespace": namespace},
            "spec": {
                "accessModes": ["ReadWriteOnce"],
                "storageClassName": claim["storageClassName"],
                "resources": {"requests": {"storage": claim["size"]}},
            },
        })
        pod["spec"]["volumes"] = [{"name": "data", "persistentVolumeClaim": {"claimName": name}}]
        container["volumeMounts"] = [{"name": "data", "mountPath": "/mnt/data"}]

    documents.append(pod)
    return "\n---\n".join(yaml.safe_dump(d) for d in documents)


def collect(context: str, namespace: str, name: str) -> dict:
    """Read the pod exactly as the platform's runtime reads it."""
    raw = kubectl(context, "-n", namespace, "get", "pod", name, "-o", "json", check=False)
    status = {"phase": "", "reason": "", "message": "", "restartCount": 0}
    if raw.strip():
        pod = json.loads(raw)
        pod_status = pod.get("status", {})
        status["phase"] = pod_status.get("phase", "")
        status["reason"] = pod_status.get("reason", "")
        status["message"] = pod_status.get("message", "")
        containers = pod_status.get("containerStatuses") or []
        if containers:
            status["restartCount"] = containers[0].get("restartCount", 0)
            # A pod whose container is waiting in a back-off is not "Running"
            # to a reader, whatever the phase says.
            waiting = (containers[0].get("state") or {}).get("waiting") or {}
            terminated = (containers[0].get("state") or {}).get("terminated") or {}
            if waiting and not status["reason"]:
                status["reason"] = waiting.get("reason", "")
                status["message"] = waiting.get("message", "")
            elif terminated and not status["reason"]:
                status["reason"] = terminated.get("reason", "")

    events_raw = kubectl(
        context, "-n", namespace, "get", "events",
        "--field-selector", f"involvedObject.name={name}", "-o", "json", check=False)
    events = []
    for item in (json.loads(events_raw).get("items", []) if events_raw.strip() else []):
        events.append({
            "reason": item.get("reason", ""),
            "message": item.get("message", ""),
            "type": item.get("type", ""),
            "at": item.get("lastTimestamp") or item.get("eventTime") or "",
            "count": item.get("count", 1),
        })
    events.sort(key=lambda event: event["at"] or "")
    return {"status": status, "events": events}


def settled(evidence: dict, incident: dict) -> bool:
    """True once the evidence shows what the incident is about.

    Waiting a fixed delay would capture a pod mid-pull as often as a pod that
    has failed, and the fixture would then encode a race rather than a cause.
    """
    expect = incident["expect"]
    reasons = {event["reason"] for event in evidence["events"]}
    phase = evidence["status"]["phase"].lower()
    wanted = incident["pod"].get("waitForRestarts")
    if wanted:
        return evidence["status"]["restartCount"] >= wanted
    if expect["detail"] == "no_capacity":
        return "FailedScheduling" in reasons
    if expect["detail"] == "image_unavailable":
        return bool(reasons & {"Failed", "ErrImagePull", "BackOff"})
    if expect["detail"] == "storage_unavailable":
        return bool(reasons & {"FailedMount", "FailedAttachVolume"})
    if expect["detail"] == "environment_failed":
        return phase == "failed"
    return phase == "running"


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--context", required=True, help="kubectl context; the DC is noryx-test")
    parser.add_argument("--only", help="capture a single incident by id")
    parser.add_argument("--out", type=pathlib.Path, default=DEFAULT_OUT)
    parser.add_argument("--timeout", type=int, default=240, help="seconds to wait for one incident to settle")
    parser.add_argument("--keep", action="store_true", help="leave the namespace in place for inspection")
    args = parser.parse_args()

    corpus = yaml.safe_load(CORPUS.read_text(encoding="utf-8"))
    namespace = corpus["namespace"]
    incidents = [i for i in corpus["incidents"] if not args.only or i["id"] == args.only]
    if not incidents:
        raise SystemExit(f"no incident named {args.only}")

    args.out.mkdir(parents=True, exist_ok=True)
    kubectl(args.context, "create", "namespace", namespace, check=False)

    failures = []
    for incident in incidents:
        name = incident["id"]
        print(f"-- {name}: {incident['label']}")
        kubectl(args.context, "-n", namespace, "delete", "pod", name, "--ignore-not-found", "--now", check=False)
        kubectl(args.context, "-n", namespace, "delete", "pvc", name, "--ignore-not-found", check=False)
        kubectl(args.context, "apply", "-f", "-", stdin=pod_manifest(incident, namespace))

        deadline = time.time() + args.timeout
        evidence = {}
        while time.time() < deadline:
            evidence = collect(args.context, namespace, name)
            if settled(evidence, incident):
                break
            time.sleep(5)
        else:
            failures.append(f"{name}: never showed its cause within {args.timeout}s")

        payload = {
            "id": name,
            "label": incident["label"],
            "expect": incident["expect"],
            "recorded": "launching",
            **evidence,
        }
        path = args.out / f"{name}.json"
        path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
        reasons = ", ".join(sorted({e["reason"] for e in evidence.get("events", [])})) or "aucun"
        print(f"   phase={evidence.get('status', {}).get('phase', '?')} evenements={reasons}")
        print(f"   -> {path.relative_to(pathlib.Path.cwd()) if str(path).startswith(str(pathlib.Path.cwd())) else path}")

        kubectl(args.context, "-n", namespace, "delete", "pod", name, "--ignore-not-found", "--now", check=False)
        kubectl(args.context, "-n", namespace, "delete", "pvc", name, "--ignore-not-found", check=False)

    if not args.keep:
        kubectl(args.context, "delete", "namespace", namespace, "--ignore-not-found", check=False)

    if failures:
        print("\n".join(failures), file=sys.stderr)
        raise SystemExit(1)
    print(f"\n{len(incidents)} incidents captured.")


if __name__ == "__main__":
    main()
