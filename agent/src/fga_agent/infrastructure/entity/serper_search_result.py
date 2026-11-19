"""Serper's `/search` wire shape. It exists only inside the Serper adapter."""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class SerperResult:
    link: str
    title: str
    snippet: str

    @staticmethod
    def from_response(body: dict) -> list[SerperResult]:
        return [SerperResult(x["link"], x.get("title", ""), x.get("snippet", "")) for x in body.get("organic", []) if x.get("link")]
