"""Estimate-reading eval against the live model: does the Agent understand a message in English, Vietnamese and Japanese?

    uv run python eval/estimate_run.py [--rules on|off|both] [--model qwen3:latest] [--limit N] [--only id,id] [-v]
                                                                     (needs `make engine-grpc` for the catalog, and Ollama)

Each case in estimate_golden.json is one message (optionally with the question it answers) and the fields it must produce; `null`
lists fields that must stay empty (the unprompted-guess checks). `--rules both` runs every case with the keyword rules on and off,
which shows what each rule still buys: delete a rule when "off" is as good as "on". The plan is a fixed summary (the first apartment of
the sample sheet: r1 Balcony, r2/r3 Bedrooms, r4 Bath, r5 Living, r6 Wc, five fixtures), so the case ids of the room checks are stable.
Compare with specs/re-architecture-system/baseline/eval-estimate.txt."""

from __future__ import annotations

import argparse
import json
import os
import sys
import time
from pathlib import Path

from fga_agent.domain.errors import EngineUnavailable, LlmUnavailable
from fga_agent.domain.model.call_context import CallContext
from fga_agent.domain.model.estimate_session import Message, SessionState
from fga_agent.domain.model.extracted import Extracted
from fga_agent.domain.model.plan_summary import PlanSummary, SummaryFixture, SummaryOpening, SummaryRoom
from fga_agent.infrastructure.engine_grpc.blueprint_engine import GrpcBlueprintEngine, open_channel
from fga_agent.infrastructure.ollama import OllamaLanguageModel
from fga_agent.service.estimate_reader import EstimateReaderService
from fga_agent.service.helper.claim_checker import ClaimCheckerService
from fga_agent.service.helper.design_catalog_loader import DesignCatalogLoaderService
from fga_agent.service.helper.keyword_reader import KeywordReaderService

HERE = Path(__file__).parent
CTX = CallContext(request_id="eval")
PLAN = PlanSummary(
    rooms=[SummaryRoom("r1", "Balcony", "balcony", 4.6), SummaryRoom("r2", "Bedroom 1", "bedroom", 11.2),
           SummaryRoom("r3", "Bedroom 2", "bedroom", 11.5), SummaryRoom("r4", "Bath", "bathroom", 5.7),
           SummaryRoom("r5", "Living / Dining / Kitchen", "living", 40.0), SummaryRoom("r6", "Wc", "bathroom", 4.7)],
    fixtures=[SummaryFixture(i, k, r) for i, k, r in [("f1", "toilet", "r6"), ("f2", "basin", "r6"), ("f3", "toilet", "r4"),
                                                      ("f4", "shower", "r4"), ("f5", "basin", "r4")]],
    openings=[SummaryOpening("d1", "entrance", ["outside", "r5"], 1.0)],
    pending_fixtures=["f1", "f2", "f3", "f4", "f5"])


def same_place(want: str, got: str, norm) -> bool:
    """'Danang' = 'Da Nang', 'Quận 7' = 'District 7': accents, spaces and the word for district do not matter, numbers do."""
    n = lambda x: "".join(ch for ch in norm(x) if ch.isalnum())  # noqa: E731
    nums = lambda x: "".join(ch for ch in x if ch.isdigit())  # noqa: E731
    if nums(want) or nums(got):
        return nums(want) == nums(got)
    return n(want) in n(got) or n(got) in n(want)


def check(case: dict, ex: Extracted, norm) -> list[str]:
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
            if got is None or not same_place(want, got, norm):
                miss.append(f"{k}={got!r} want {want!r}")
        elif got != want:
            miss.append(f"{k}={got!r} want {want!r}")
    for k in case.get("null", []):
        got = getattr(ex, k)
        if got not in (None, False, []):
            miss.append(f"{k}={got!r} should be empty")
    return miss


def run(reader: EstimateReaderService, cases: list[dict], verbose: bool) -> tuple[int, float, float]:
    from unicodedata import normalize

    norm = lambda x: "".join(c for c in normalize("NFKD", x.casefold()) if not 0x300 <= ord(c) <= 0x36F)  # noqa: E731
    ok, total_t, slow = 0, 0.0, 0.0
    for c in cases:
        state, history = SessionState(), []
        if c.get("pending"):
            state.pending = [c["pending"]]
            history = [Message("assistant", c["question"])]
        t = time.time()
        try:
            miss = check(c, reader.read(CTX, PLAN, state, c["message"], history), norm)
        except (LlmUnavailable, EngineUnavailable) as e:
            miss = [f"error: {e}"]
        dt = time.time() - t
        total_t, slow, ok = total_t + dt, max(slow, dt), ok + (not miss)
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
    llm = OllamaLanguageModel(text_model=a.model)
    engine = GrpcBlueprintEngine(open_channel(os.environ.get("ENGINE_ADDR", "localhost:8092")))
    catalog, keywords, checker = DesignCatalogLoaderService(engine), KeywordReaderService(), ClaimCheckerService()
    print(f"model {llm.text_model}, {len(cases)} messages")
    worst = 0
    for rules in ([True, False] if a.rules == "both" else [a.rules == "on"]):
        print(f"\nkeyword rules {'ON ' if rules else 'OFF'}")
        ok, avg, slow = run(EstimateReaderService(llm, catalog, checker, keywords, rules), cases, a.v)
        print(f"  -> {ok}/{len(cases)} read correctly, {avg:.1f}s per message (slowest {slow:.1f}s)")
        worst = max(worst, len(cases) - ok)
    return 1 if worst else 0


if __name__ == "__main__":
    sys.exit(main())
