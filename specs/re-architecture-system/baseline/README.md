# Baselines (T0)

Recorded on the untouched stack, 2026-11-19, before any code moved. Later tasks compare against these.

| File | What | How to refresh the comparison |
| --- | --- | --- |
| `replay-before.txt` | Status and JSON *shape* (sorted key paths) of ~33 blueprint/estimate routes (non-LLM) | `scripts/smoke-replay.sh /tmp/after.txt && diff replay-before.txt /tmp/after.txt` (stable across runs; verified identical twice) |
| `eval-brief.txt` | `make engine-eval` (15 golden prompts, qwen3:latest) | `make agent-eval` after T13b |
| `eval-estimate.txt` | `make engine-eval-estimate` | `make agent-eval-estimate` after T13b |
| `../test-map.md` | The 210 collected pytest node IDs | `cd blueprint-engine && uv run pytest --collect-only -q` |

The LLM steps of the replay (`OLLAMA=1 scripts/smoke-replay.sh`) are not part of the saved baseline: they depend on the model's answers and take minutes. Run them by hand around each switch.

## Eval baseline vs the earlier numbers

On 2026-11-13 the brief eval was extraction 15/15, end-to-end 14/15 (avg 28 s). Today the same eval on the same model gave **extraction 14/15, end-to-end 13/15** (avg 20.6 s); the failing case is `too_big` (the model returned no rooms). The model is not deterministic, so the gate for the later tasks is "within one case of today's numbers", not the older, better ones. See requirements criterion 8 (amended 2026-11-19).
