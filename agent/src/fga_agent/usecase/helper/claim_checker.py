from __future__ import annotations

from typing import Protocol


class ClaimChecker(Protocol):
    """Checks the model's claims against the person's own words. A claim is {quote, field, value}: the quote is the message's
    words copied, the field names what it says, the value is what it says. A claim whose quote is not in the message, or whose
    value is not valid for its field, is dropped, so a small model cannot invent what nobody said. Nothing here knows a language."""

    def grounded(self, quote: str, message: str) -> bool:
        """The quote is in the message: accent, case, width and spacing insensitive, with a small tolerance (85% of the quote in
        one run) for a model that copies loosely."""

    def verified(self, fields: dict[str, tuple[str, bool]], claims: list[dict], message: str, rate_codes: list[str]) -> dict:
        """The values of the claims that pass, by reading field. `fields` maps a claim field to (reading field, takes several
        values). One bad value never costs the others."""
