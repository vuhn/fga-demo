"""Estimate-agent reading eval against the live LLM: does it understand a message in English, Vietnamese and Japanese?

    uv run python eval/estimate_run.py [--rules on|off|both] [--model qwen3:latest] [--limit N] [--only id,id] [-v]

Each case in estimate_golden.json is one message (optionally with the question it answers) and the fields it must produce;
`null` lists fields that must stay empty (the unprompted-guess checks). `--rules both` runs every case with the keyword rules
on and off, which shows what each rule still buys: delete a rule when "off" is as good as "on". Run it whenever the model or
a reading prompt changes."""

from __future__ import annotations

import argparse
import json
import sys
import time
from pathlib import Path

HERE = Path(__file__).parent
sys.path[:0] = [str(HERE.parent / "tests"), str(HERE.parent / "src")]

import sample_dxf  # noqa: E402
from fga_engine import estimate_agent as ag  # noqa: E402
from fga_engine.dxf_plan import import_dxf  # noqa: E402
from fga_engine.estimate_reading import _norm  # noqa: E402
from fga_engine.estimate_models import Message, SessionState  # noqa: E402
from fga_engine.llm import LLMError, OllamaClient  # noqa: E402
from fga_engine.domain.schemas import PlacedFixture  # noqa: E402


def plan():
    m = import_dxf(sample_dxf.sheet()[0]).plans[0].model  # r1 Balcony, r2/r3 Bedrooms, r4 Bath, r5 Living, r6 Wc
    fx = [("f1", "toilet", "r6"), ("f2", "basin", "r6"), ("f3", "toilet", "r4"), ("f4", "shower", "r4"), ("f5", "basin", "r4")]
    return m.model_copy(update={"fixtures": [PlacedFixture(id=i, kind=k, room_id=r, rect=(0, 0, 0.5, 0.5), facing="south") for i, k, r in fx]})


def same_place(want: str, got: str) -> bool:
    """'Danang' = 'Da Nang', 'Quận 7' = 'District 7': accents, spaces and the word for district do not matter, numbers do."""
    n = lambda x: "".join(ch for ch in _norm(x) if ch.isalnum())  # noqa: E731
    nums = lambda x: "".join(ch for ch in x if ch.isdigit())  # noqa: E731
    if nums(want) or nums(got):
        return nums(want) == nums(got)
    return n(want) in n(got) or n(got) in n(want)


def check(case: dict, ex: ag.Extracted) -> list[str]:
    miss: list[str] = []
    for k, want in case["expect"].items():
        got = getattr(ex, k)
        if k == "scope_rooms":
            got = {r.room_id: r.state for r in got}
            if got != want:
                miss.append(f"{k}={got} want {want}")
        elif isinstance(want, list):
            if set(got) != set(want):
                miss.append(f"{k}={got} want {want}")
        elif isinstance(want, str) and k in ("city", "district", "explain"):
            if got is None or not same_place(want, got):
                miss.append(f"{k}={got!r} want {want!r}")
        elif got != want:
            miss.append(f"{k}={got!r} want {want!r}")
    for k in case.get("null", []):
        got = getattr(ex, k)
        if got not in (None, False, []):
            miss.append(f"{k}={got!r} should be empty")
    return miss


def run(llm, cases: list[dict], rules: bool, verbose: bool) -> tuple[int, float, float]:
    ag.RULES = rules
    model = plan()
    ok, total_t, slow = 0, 0.0, 0.0
    for c in cases:
        state, hist = SessionState(), []
        if c.get("pending"):
            state.pending = [c["pending"]]
            hist = [Message(role="assistant", content=c["question"])]
        t = time.time()
        try:
            ex = ag._extract(llm, model, state, c["message"], hist)
            miss = check(c, ex)
        except LLMError as e:
            miss = [f"LLM error: {e}"]
        dt = time.time() - t
        total_t += dt
        slow = max(slow, dt)
        ok += not miss
        if miss or verbose:
            print(f"  {'PASS' if not miss else 'FAIL'}  {c['id']:<24} {dt:4.1f}s  {'; '.join(miss)}")
    return ok, total_t / max(len(cases), 1), slow


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--rules", default="both", choices=["on", "off", "both"])
    ap.add_argument("--model", default=None)
    ap.add_argument("--limit", type=int, default=0)
    ap.add_argument("--only", default="")
    ap.add_argument("-v", action="store_true")
    a = ap.parse_args()
    cases = json.loads((HERE / "estimate_golden.json").read_text())
    if a.only:
        cases = [c for c in cases if c["id"] in a.only.split(",")]
    if a.limit:
        cases = cases[: a.limit]
    llm = OllamaClient(text_model=a.model)
    print(f"model {llm.text_model}, {len(cases)} messages")
    worst = 0
    for rules in ([True, False] if a.rules == "both" else [a.rules == "on"]):
        print(f"\nkeyword rules {'ON ' if rules else 'OFF'}")
        ok, avg, slow = run(llm, cases, rules, a.v)
        print(f"  -> {ok}/{len(cases)} read correctly, {avg:.1f}s per message (slowest {slow:.1f}s)")
        worst = max(worst, len(cases) - ok)
    return 1 if worst else 0


if __name__ == "__main__":
    sys.exit(main())
