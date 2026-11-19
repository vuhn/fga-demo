"""Golden-prompt eval against the live model and the live Engine. Run whenever the model or a prompt changes.

    uv run python eval/run.py [--model qwen3:latest] [--limit N] [--enhance]      (needs `make engine` and Ollama)

Reports, per prompt: brief latency, whether a plan was built, the error checks and the Engine's notes; then the overall pass rate.
Expectations in golden.json are loose (room counts by type, lot, entrance) because the model's job is extraction only. Compare with
specs/re-architecture-system/baseline/eval-brief.txt: the gate is within one case."""

from __future__ import annotations

import argparse
import json
import os
import sys
import time
from collections import Counter
from pathlib import Path

from fga_agent.domain.errors import EngineUnavailable, InvalidEdit, LlmUnavailable
from fga_agent.domain.model.call_context import CallContext
from fga_agent.infrastructure.engine_grpc.blueprint_engine import GrpcBlueprintEngine, open_channel
from fga_agent.infrastructure.ollama import OllamaLanguageModel
from fga_agent.service.brief_drafter import BriefDrafterService
from fga_agent.service.helper.design_catalog_loader import DesignCatalogLoaderService
from fga_agent.service.helper.feedback_asker import FeedbackAskerService

CTX = CallContext(request_id="eval")


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--model", default=None)
    ap.add_argument("--limit", type=int, default=0)
    ap.add_argument("--enhance", action="store_true", help="run the Enhancer step before drafting")
    args = ap.parse_args()
    cases = json.loads((Path(__file__).parent / "golden.json").read_text())
    if args.limit:
        cases = cases[: args.limit]
    llm = OllamaLanguageModel(text_model=args.model)
    engine = GrpcBlueprintEngine(open_channel(os.environ.get("ENGINE_ADDR", "localhost:8092")))
    drafter = BriefDrafterService(engine, llm, DesignCatalogLoaderService(engine), FeedbackAskerService(llm))
    print(f"model {llm.text_model}, {len(cases)} prompts, enhancer {'on' if args.enhance else 'off'}")
    passed = extracted = 0
    times: list[float] = []
    for case in cases:
        t = time.time()
        try:
            draft = drafter.draft(CTX, case["prompt"], args.enhance, lambda p: None)
        except (LlmUnavailable, InvalidEdit, EngineUnavailable) as e:
            print(f"  FAIL  {case['id']:<22} brief error: {e}")
            continue
        dt = time.time() - t
        times.append(dt)
        brief = json.loads(draft.brief)
        build = engine.generate(CTX, draft.brief)
        got = Counter(r["type"] for r in brief["rooms"])
        exp = case.get("expect", {})
        misses = [f"{k}={got.get(k, 0)}≠{v}" for k, v in exp.get("types", {}).items() if got.get(k, 0) != v]
        if "lot" in exp and [brief["lot_width"], brief["lot_depth"]] != exp["lot"]:
            misses.append(f"lot={brief['lot_width']}x{brief['lot_depth']}")
        if "entrance" in exp and brief.get("entrance_side") != exp["entrance"]:
            misses.append(f"entrance={brief.get('entrance_side')}")
        built = bool(build.model) and build.ok
        ok_extract, ok_build = not misses, built == exp.get("fits", True)
        extracted += ok_extract
        passed += ok_extract and ok_build
        detail = "; ".join(misses + ([] if ok_build else [f"built={built} expected={exp.get('fits', True)}"]) + build.issues[-1:] + draft.notes)
        print(f"  {'PASS' if ok_extract and ok_build else 'FAIL'}  {case['id']:<22} {dt:5.1f}s  {detail}")
    n = len(cases)
    print(f"\nextraction {extracted}/{n}, end-to-end {passed}/{n}, avg brief latency {sum(times) / max(len(times), 1):.1f}s")
    return 0 if passed == n else 1


if __name__ == "__main__":
    sys.exit(main())
