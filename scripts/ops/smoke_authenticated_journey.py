#!/usr/bin/env python3
"""
Signs in, and checks the platform actually lets somebody in.

Two defects this week were invisible to every existing check because both left
the platform *answering correctly*: a workspace that launched and then refused
entry, and an interface that never created the session cookie the proxy needs.
An unauthenticated smoke cannot see either. This one signs in with a real
account and asks the questions a user would.

    NORYX_SMOKE_USER=... NORYX_SMOKE_PASSWORD=... \
    BASE_URL=https://datalab.example.local \
    python3 scripts/ops/smoke_authenticated_journey.py

Deliberately short. Full browser suites rot: they grow until they are slow,
then flaky, then ignored. This one checks that signing in works, that the
session reaches the API, and that the workspace list loads - the path every
user takes before doing anything at all.
"""

import os
import sys

from playwright.sync_api import TimeoutError as PlaywrightTimeout
from playwright.sync_api import sync_playwright

BASE_URL = os.getenv("BASE_URL", "https://datalab.example.local").rstrip("/")
USERNAME = os.getenv("NORYX_SMOKE_USER", "")
PASSWORD = os.getenv("NORYX_SMOKE_PASSWORD", "")
TIMEOUT = int(os.getenv("NORYX_SMOKE_TIMEOUT_MS", "30000"))


def check(label: str, condition: bool, detail: str = "") -> bool:
    print(f"  {'ok  ' if condition else 'FAIL'}  {label}{'' if condition else ': ' + detail}")
    return condition


def main() -> int:
    if not USERNAME or not PASSWORD:
        print("NORYX_SMOKE_USER and NORYX_SMOKE_PASSWORD are required", file=sys.stderr)
        return 2

    print(f"Authenticated journey: {BASE_URL} as {USERNAME}")
    failures = []

    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(headless=True)
        context = browser.new_context(ignore_https_errors=True)
        page = context.new_page()

        page.goto(BASE_URL + "/", wait_until="domcontentloaded", timeout=TIMEOUT)
        # A signed-out visitor gets a card with one button, not the application
        # shell: the shell and its account menu only exist once past the gate.
        page.get_by_test_id("sign-in").click(timeout=TIMEOUT)

        # Keycloak's own form. Its field names are part of its HTTP contract,
        # unlike its labels, which are themed per realm.
        page.wait_for_selector("#username", timeout=TIMEOUT)
        page.fill("#username", USERNAME)
        page.fill("#password", PASSWORD)
        page.click("#kc-login", timeout=TIMEOUT)

        try:
            page.wait_for_url(BASE_URL + "/**", timeout=TIMEOUT)
        except PlaywrightTimeout:
            failures.append("the browser never came back from the identity provider")

        # Signed in means the menu now offers signing out. Asserting on the
        # identity rather than on a page having rendered: a page renders while
        # signed out too.
        page.get_by_test_id("account-menu").click(timeout=TIMEOUT)
        signed_in = page.get_by_test_id("sign-out").count() == 1
        if not check("signed in", signed_in, "the menu still offers signing in"):
            failures.append("sign-in")
        page.keyboard.press("Escape")

        # The session must reach the API. This is the check that would have
        # caught the interface that authenticated and then never created the
        # cookie the proxy needs: the screen looked fine, and every workspace
        # refused entry.
        response = page.request.get(BASE_URL + "/api/v1/projects")
        if not check("the session reaches the API", response.status == 200, f"HTTP {response.status}"):
            failures.append("api")

        # The list every user opens first. A 200 with an error body is still a
        # failure here, which is why the payload is looked at.
        workspaces = page.request.get(BASE_URL + "/api/v1/workspaces")
        payload = workspaces.json() if workspaces.status == 200 else {}
        if not check(
            "the workspace list loads",
            workspaces.status == 200 and "items" in payload,
            f"HTTP {workspaces.status}",
        ):
            failures.append("workspaces")

        browser.close()

    if failures:
        print(f"\nJourney failed: {', '.join(failures)}", file=sys.stderr)
        return 1
    print("\nThe journey holds")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as error:  # noqa: BLE001 - the message is the point
        print(f"Journey failed: {error}", file=sys.stderr)
        raise SystemExit(1)
