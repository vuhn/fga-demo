from __future__ import annotations

import html
import re
import urllib.robotparser
from urllib.parse import urlparse

import httpx

USER_AGENT = "fga-agent-price-research/0.1"
MAX_BYTES = 2_000_000


def page_text(raw: str) -> str:
    raw = re.sub(r"(?is)<(script|style|noscript)[^>]*>.*?</\1>", " ", raw)
    return re.sub(r"\s+", " ", html.unescape(re.sub(r"(?s)<[^>]+>", " ", raw))).strip()


class HttpWebPages:
    """Reads a public page politely: only http(s), only what the site's robots.txt allows, only text, never a huge file."""

    def __init__(self, http: httpx.Client | None = None) -> None:
        self._http = http or httpx.Client(timeout=12, follow_redirects=True, headers={"User-Agent": USER_AGENT})

    def text(self, url: str) -> str | None:
        try:
            u = urlparse(url)
            if u.scheme not in ("http", "https"):
                return None
            robots = self._http.get(f"{u.scheme}://{u.netloc}/robots.txt")
            if robots.status_code == 200:
                rules = urllib.robotparser.RobotFileParser()
                rules.parse(robots.text.splitlines())
                if not rules.can_fetch(USER_AGENT, url):
                    return None
            r = self._http.get(url)
            if r.status_code != 200 or "text" not in r.headers.get("content-type", "") or len(r.content) > MAX_BYTES:
                return None
            return page_text(r.text)
        except (httpx.HTTPError, ValueError):
            return None
