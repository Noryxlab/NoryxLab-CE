#!/usr/bin/env python3
"""
Noryx CE smoke test: frontend versions + Keycloak login click redirect.

Usage:
  BASE_URL=https://datalab.noryxlab.ai python3 scripts/ops/smoke_frontend_login.py
"""

import json
import os
import re
import sys
import urllib.request

from playwright.sync_api import sync_playwright


def get_json(url: str):
    with urllib.request.urlopen(url, timeout=20) as resp:
        if resp.status != 200:
            raise RuntimeError(f"{url} returned status {resp.status}")
        return json.loads(resp.read().decode("utf-8"))


def main() -> int:
    base_url = os.getenv("BASE_URL", "https://datalab.noryxlab.ai").rstrip("/")
    front_version_url = f"{base_url}/version.json"
    back_version_url = f"{base_url}/api/v1/version"

    front_v = get_json(front_version_url).get("version", "")
    back_raw = get_json(back_version_url)
    back_v = back_raw.get("backendVersion") or back_raw.get("version", "")

    if not front_v:
        raise RuntimeError("frontend version missing in /version.json")
    if not back_v:
        raise RuntimeError("backend version missing in /api/v1/version")

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        ctx = browser.new_context(ignore_https_errors=True)
        page = ctx.new_page()

        page.goto(base_url + "/", wait_until="domcontentloaded", timeout=60000)
        page.wait_for_timeout(1500)

        versions_text = page.locator("#versions").inner_text()
        if front_v not in versions_text or back_v not in versions_text:
            raise RuntimeError(
                f"versions badge mismatch: badge='{versions_text}', expected front='{front_v}', back='{back_v}'"
            )

        if page.locator("#login").count() != 1:
            raise RuntimeError("login button '#login' not found")

        page.click("#login", timeout=10000)
        page.wait_for_timeout(3000)
        current_url = page.url

        if not re.search(r"/auth/realms/.+/protocol/openid-connect/auth", current_url):
            raise RuntimeError(f"login click did not redirect to keycloak auth URL, got '{current_url}'")

        browser.close()

    print("SMOKE_OK")
    print(f"front={front_v}")
    print(f"back={back_v}")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as err:
        print(f"SMOKE_FAIL: {err}", file=sys.stderr)
        raise SystemExit(1)
