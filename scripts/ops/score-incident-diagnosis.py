#!/usr/bin/env python3
"""Score an assistant's diagnosis against incidents whose cause is known.

The question a support agent has to answer is not "does it write well". It is:
given the evidence the platform already produces, does it name the cause that
actually applied, and does it say so about a healthy system too.

The corpus comes from scripts/ops/capture-incidents.py: real failures caused on
purpose on a real cluster, so the answer is known by construction rather than
by a human's later opinion.

Speaks plain OpenAI, so it scores the llmaas gateway, a vLLM server, or a
hosted API without knowing the difference:

    scripts/ops/score-incident-diagnosis.py --base-url http://127.0.0.1:8081/v1 \
        --api-key llmaas_... --model fast
"""

from __future__ import annotations

import argparse
import json
import pathlib
import statistics
import time
import urllib.error
import urllib.request

HERE = pathlib.Path(__file__).resolve().parent
FIXTURES = HERE.parents[1] / "backend" / "internal" / "http" / "handlers" / "testdata" / "incidents"

# The vocabulary the platform itself emits. The assistant answers in it, so a
# score measures the diagnosis rather than a synonym-matching exercise.
CAUSES = {
    "image_unavailable": "the environment image cannot be pulled from the registry",
    "no_capacity": "no node has enough free CPU or memory",
    "storage_unavailable": "a volume could not be provisioned, attached or mounted",
    "environment_failed": "the environment started and exited",
    "environment_restarting": "the environment restarts in a loop",
    "healthy": "nothing is wrong; the workspace is starting or running normally",
}

SYSTEM = (
    "You are a platform support engineer for a data-science platform running on Kubernetes. "
    "You are given the evidence the platform collected about one workspace that a user reports "
    "as not starting. Name the single cause. Answer with a JSON object and nothing else:\n"
    '{"cause": "<one of: ' + ", ".join(CAUSES) + '>", "evidence": "<the one line from the evidence that proves it>", '
    '"action": "<the next thing a human should do, one sentence>"}\n'
    "Causes and what they mean:\n" + "\n".join(f"- {k}: {v}" for k, v in CAUSES.items()) + "\n"
    "If the evidence shows a normally starting or running workspace, the cause is healthy. "
    "Never invent evidence that is not in the input."
)


def prompt_for(fixture: dict) -> str:
    lines = [
        f"Workspace status recorded by the platform: {fixture['recorded']}",
        f"Pod phase: {fixture['status'].get('phase') or 'unknown'}",
    ]
    if fixture["status"].get("reason"):
        lines.append(f"Pod reason: {fixture['status']['reason']}")
    if fixture["status"].get("message"):
        lines.append(f"Pod message: {fixture['status']['message']}")
    lines.append(f"Container restarts: {fixture['status'].get('restartCount', 0)}")
    lines.append("Events recorded by Kubernetes, oldest first:")
    for event in fixture["events"]:
        lines.append(f"  [{event['type']}] {event['reason']}: {event['message']}")
    return "\n".join(lines)


def ask(base_url: str, api_key: str, model: str, prompt: str, timeout: int) -> tuple[str, float]:
    body = json.dumps({
        "model": model,
        "messages": [
            {"role": "system", "content": SYSTEM},
            {"role": "user", "content": prompt},
        ],
        "temperature": 0,
        "max_tokens": 300,
    }).encode()
    request = urllib.request.Request(
        base_url.rstrip("/") + "/chat/completions", data=body,
        headers={"Content-Type": "application/json", "Authorization": f"Bearer {api_key}"})
    started = time.time()
    with urllib.request.urlopen(request, timeout=timeout) as response:
        payload = json.load(response)
    return payload["choices"][0]["message"]["content"], time.time() - started


def parse_cause(answer: str) -> tuple[str, str, str]:
    """Read the model's answer, tolerating the fences it wraps JSON in."""
    text = answer.strip()
    if "```" in text:
        text = text.split("```")[1]
        if text.startswith("json"):
            text = text[4:]
    start, end = text.find("{"), text.rfind("}")
    if start >= 0 and end > start:
        try:
            parsed = json.loads(text[start:end + 1])
            return (str(parsed.get("cause", "")).strip(),
                    str(parsed.get("evidence", "")).strip(),
                    str(parsed.get("action", "")).strip())
        except json.JSONDecodeError:
            pass
    # An unparseable answer is a wrong answer: something has to act on it.
    return "", "", ""


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", required=True)
    parser.add_argument("--api-key", default="none")
    parser.add_argument("--model", default="fast")
    parser.add_argument("--repeat", type=int, default=1, help="runs per incident; a support agent must be consistent, not lucky once")
    parser.add_argument("--timeout", type=int, default=600)
    parser.add_argument("--out", type=pathlib.Path)
    args = parser.parse_args()

    fixtures = sorted(FIXTURES.glob("*.json"))
    if not fixtures:
        raise SystemExit(f"no fixtures in {FIXTURES}: capture them with capture-incidents.py")

    results = []
    for path in fixtures:
        fixture = json.loads(path.read_text(encoding="utf-8"))
        expected = fixture["expect"]["detail"] or "healthy"
        prompt = prompt_for(fixture)
        for attempt in range(args.repeat):
            try:
                answer, elapsed = ask(args.base_url, args.api_key, args.model, prompt, args.timeout)
            except (urllib.error.URLError, TimeoutError) as error:
                results.append({"id": fixture["id"], "expected": expected, "got": "", "correct": False,
                                "seconds": None, "error": str(error)})
                print(f"{fixture['id']:24} ERREUR {error}")
                continue
            cause, evidence, action = parse_cause(answer)
            correct = cause == expected
            results.append({"id": fixture["id"], "expected": expected, "got": cause,
                            "correct": correct, "seconds": round(elapsed, 1),
                            "evidence": evidence, "action": action, "raw": answer})
            mark = "OK  " if correct else "FAUX"
            suffix = "" if args.repeat == 1 else f" [{attempt + 1}/{args.repeat}]"
            print(f"{fixture['id']:24} {mark} attendu={expected:22} obtenu={cause or '(illisible)'}{suffix}  {elapsed:.1f}s")

    answered = [r for r in results if r.get("seconds") is not None]
    correct = [r for r in results if r["correct"]]
    print()
    print(f"diagnostics justes : {len(correct)}/{len(results)}")
    if answered:
        times = [r["seconds"] for r in answered]
        print(f"temps median       : {statistics.median(times):.1f}s")
    # Reported separately because the two failures are not the same failure: a
    # false alarm on a healthy system is what makes an agent unusable.
    false_alarms = [r for r in results if r["expected"] == "healthy" and not r["correct"]]
    if false_alarms:
        print(f"fausses alertes    : {len(false_alarms)} - un agent qui trouve une panne sur un systeme sain coute plus cher que pas d'agent")

    if args.out:
        args.out.write_text(json.dumps(results, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
        print(f"detail -> {args.out}")


if __name__ == "__main__":
    main()
