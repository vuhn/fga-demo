"""Golden-prompt eval against the live LLM. Run whenever the model or a prompt changes.

    uv run python eval/run.py [--model qwen3:latest] [--limit N]

Reports, per prompt: brief latency, whether a model was produced, error checks passed,
and shrink notes; then the overall pass rate. Expectations in golden.json are loose
(room counts by type, lot, entrance) because the LLM's job is extraction only.
"""

from __future__ import annotations

import argparse
import json
import sys
import time
from collections import Counter
from pathlib import Path

from blueprint_engine import llm_steps
from blueprint_engine.checks import failed
from blueprint_engine.llm import LLMError, OllamaClient


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--model", default=None)
    ap.add_argument("--limit", type=int, default=0)
    ap.add_argument("--enhance", action="store_true", help="run the Enhancer step before drafting")
    args = ap.parse_args()
    cases = json.loads((Path(__file__).parent / "golden.json").read_text())
    if args.limit:
        cases = cases[: args.limit]
    llm = OllamaClient(text_model=args.model)
    print(f"model {llm.text_model}, {len(cases)} prompts, enhancer {'on' if args.enhance else 'off'}")
    passed = extracted = 0
    times: list[float] = []
    for case in cases:
        t = time.time()
        try:
            brief, notes = llm_steps.brief_from_text(llm, case["prompt"], use_enhancer=args.enhance)
        except (LLMError, ValueError) as e:
            print(f"  FAIL  {case['id']:<22} brief error: {e}")
            continue
        dt = time.time() - t
        times.append(dt)
        step = llm_steps.create(llm, brief, notes)
        res = step.result
        got = Counter(r.type for r in res.brief.rooms)
        exp = case.get("expect", {})
        misses = [f"{k}={got.get(k, 0)}≠{v}" for k, v in exp.get("types", {}).items() if got.get(k, 0) != v]
        if "lot" in exp and [brief.lot.width, brief.lot.depth] != exp["lot"]:
            misses.append(f"lot={brief.lot.width}x{brief.lot.depth}")
        if "entrance" in exp and brief.entrance_side != exp["entrance"]:
            misses.append(f"entrance={brief.entrance_side}")
        should_fit = exp.get("fits", True)
        built = res.model is not None and not failed(res.checks)
        ok_extract = not misses
        ok_build = built == should_fit
        extracted += ok_extract
        passed += ok_extract and ok_build
        status = "PASS" if ok_extract and ok_build else "FAIL"
        detail = "; ".join(misses + ([] if ok_build else [f"built={built} expected={should_fit}"]) + res.issues[-1:] + step.notes)
        print(f"  {status}  {case['id']:<22} {dt:5.1f}s  {detail}")
    n = len(cases)
    avg = sum(times) / len(times) if times else 0
    print(f"\nextraction {extracted}/{n}, end-to-end {passed}/{n}, avg brief latency {avg:.1f}s")
    return 0 if passed == n else 1


if __name__ == "__main__":
    sys.exit(main())
