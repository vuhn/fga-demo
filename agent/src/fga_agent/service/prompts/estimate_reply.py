"""What the model is asked when it writes a chat reply of the estimate conversation, and the fixed texts used when it cannot:
pure functions, inputs in, text out. The model never invents or changes a number: it is given the figures."""

from __future__ import annotations

import json
from dataclasses import asdict

from ...domain.model.estimate import Estimate, EstimateLine
from ...domain.model.rate import RateResult

REPLY_SYSTEM = (
    "You are a cost-estimation assistant talking with a person about the floor plan they uploaded. Write the next chat message: short, plain, "
    "friendly, no headings and no tables. Reply in the language of the person's last message. Do what the instruction says and nothing else. "
    "Never invent or change a number; use only the figures you are given."
)
ASK = "Ask the person this one question (you may add a short reason). Ask nothing else."
UNSUPPORTED = "Tell the person this plan cannot be estimated and why."
PRESENT = ("Present the estimate: the base total and range, the biggest packages, what is assumed or unpriced, "
           "and offer to change any answer or to explain a line. Use the figures exactly as given. If "
           "changes_applied_this_turn is given, say first what was changed, using only that list.")
EXPLAIN = "Explain this cost line: the quantity and how it was measured, the rate and where it comes from. Be concrete."
NOTHING_CHANGED = ("Nothing in the estimate was changed by this message. Never say or imply that anything was updated, removed, "
                   "recalculated or changed. Answer the question if it is one; otherwise say plainly that you could not apply it and what can "
                   "be changed: take an item out or put it back (for example 'the doors already exist'), set a unit rate, change an answer "
                   "(quality, city, scope, markups).")
NOTHING_CHANGED_FALLBACK = ("I did not change anything. I can take an item out (for example \"the doors already exist\"), set a unit "
                            "rate, or change an answer such as quality, city or markups.")
NO_PRICES = ("No unit prices could be found, so nothing is in the total. Say that plainly, say why (see unpriced and warnings), "
             "and offer to take the person's own rates.")


def reply_user(instruction: str, data: dict) -> str:
    return f"Instruction: {instruction}\n\nData:\n{json.dumps(data, ensure_ascii=False)}"


def money(v: float) -> str:
    return f"{v:,.0f} VND"


def summary(est: Estimate) -> dict:
    """What the model may say about an estimate: the totals, the biggest packages, what is assumed or unpriced."""
    top = sorted(est.packages.items(), key=lambda kv: -kv[1]["base"])[:4]
    nothing = est.total.get("base", 0) == 0
    return {**({"note": NO_PRICES} if nothing else {}),
            "total_low": money(est.total.get("low", 0)), "total_base": money(est.total.get("base", 0)),
            "total_high": money(est.total.get("high", 0)),
            "largest_packages": [{"package": k, "base": money(v["base"])} for k, v in top],
            "assumptions": est.assumptions[:6], "unpriced": [w for w in est.warnings if w.startswith("No price")][:6],
            "warnings": est.warnings[:4]}


def estimate_fallback(s: dict) -> str:
    return f"Estimated total {s['total_base']} (range {s['total_low']} to {s['total_high']}, VAT included)."


def line_context(line: EstimateLine, rate: RateResult | None) -> dict:
    sources = [{"url": x.url, "quote": x.quote, "retrieved_at": x.retrieved_at} for x in (rate.sources if rate else [])][:4]
    return {"line": asdict(line), "sources": sources}


def line_fallback(line: EstimateLine) -> str:
    return f"{line.description}: {line.quantity:g} {line.unit}. Basis: {line.basis}"
