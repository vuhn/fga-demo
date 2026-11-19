---
name: test-plan
description: Stage 6 of the feature SDLC. Produce and run the manual or end-to-end test checklist for a feature from its acceptance criteria. Use after tasks are built, before release.
---

# Test

1. From `requirements.md` acceptance criteria and `plan.md` test plan, write the checklist into the Test section of `plan.md`: one row per criterion with exact steps, actor (seed accounts ada, bob, cara, dan, eve, fay `@example.com`, password `password`), tenant header, and expected result (status code, Check result from `docs/decision-catalog.md`, UI state). Include deny cases, not only allow.
2. Automated first: `make check`, `make verify-live`, engine tests (`make engine-test`) as relevant; add missing tests rather than manual steps where cheap.
3. Run what can be run (API over HTTP via curl after `make api`; the panel via the `run` skill) and record real results, not expectations. Anything not run is marked "not run".
4. Failures become failing tests first, then fixes. Gate: user confirms the manual parts; set Test status in the feature README.
