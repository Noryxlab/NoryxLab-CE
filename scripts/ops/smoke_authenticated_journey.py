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
        # Keycloak asks for both on one page, or for the username first and the
        # password on a second - depending on the realm's flow and its theme.
        # Handling both rather than assuming, because the assumption breaks on
        # somebody else's realm and the failure reads as "login is broken".
        if page.locator("#password").count() == 0:
            page.click("#kc-login", timeout=TIMEOUT)
            page.wait_for_selector("#password", timeout=TIMEOUT)
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

        # The session must reach the API, and the check has to watch what the
        # application actually does. Replaying a request from the test's own
        # HTTP client sends the cookies and not the bearer the SPA holds in
        # memory, so it comes back 401 while the application works perfectly -
        # a false alarm that would teach whoever reads it to ignore this test.
        api_calls = []
        page.on(
            "response",
            lambda response: api_calls.append((response.url, response.status))
            if "/api/v1/" in response.url
            else None,
        )
        page.goto(BASE_URL + "/projects", wait_until="networkidle", timeout=TIMEOUT)

        answered = [(url, status) for url, status in api_calls if status == 200]
        refused = [(url, status) for url, status in api_calls if status in (401, 403)]
        if not check(
            "the application's API calls are answered",
            len(answered) > 0 and len(refused) == 0,
            f"{len(answered)} answered, {len(refused)} refused: {refused[:2]}",
        ):
            failures.append("api")

        # The proxy cookie. Its absence is the defect that let somebody sign in
        # and then be refused entry to every workspace, with nothing on screen
        # to say why: the interface authenticated and never asked the backend
        # to open a session.
        cookies = {cookie["name"] for cookie in context.cookies()}
        if not check(
            "the workspace proxy session cookie exists",
            "noryx_session" in cookies,
            f"cookies present: {sorted(cookies)}",
        ):
            failures.append("session cookie")

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
