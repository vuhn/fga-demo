---
name: monitor-triage
description: Stage 8 of the feature SDLC. Turn production errors or logs into a diagnosis and a new requirements.md so the loop restarts. Use when the user pastes errors, an incident, or asks to triage after a release.
---

# Monitor and maintain

1. Collect: the errors/logs the user provides (time window, service, tenant if known). Do not guess missing data; ask.
2. Diagnose read-only: find the code path (`api/`, engine, web), reproduce locally if possible, classify (bug, data, config, dependency, capacity). Confirm with evidence (file:line, query, log).
3. Decide with the user: fix now, schedule, or dismiss (reason recorded).
4. Write `specs/<new-or-same-feature>/requirements.md` for the fix: symptom, root cause, acceptance criteria that include a failing test reproducing it. Then continue the chain via `/feature` (small fixes may skip design stages).
5. Append a short lesson (symptom, cause, prevention) to `docs/harness/lessons.md`; if the same mistake could recur, add a rule to CLAUDE.md or a check to `make check`, and a case to the relevant eval/test.
