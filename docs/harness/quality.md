# Quality

Grades A-D. Update when a session changes a row. Fix the lowest first.

| Area | Verification | Agent legibility | Test stability | Key gap |
| --- | --- | --- | --- | --- |
| Authorization model | A (live model test + role tests, passing 2026-10-23) | A (`docs/authorization-model.md`) | B (needs live OpenFGA) | none |
| Relation builders (`infrastructure/fga`) | A (pure unit tests) | A | A | none |
| Folder / album services | C (no service-level tests; covered via model test) | A | n/a | no HTTP-level test |
| Document / photo services | C | A | n/a | no HTTP-level test |
| Members / roles services | C | A | n/a | no HTTP-level test |
| HTTP handlers | D (untested) | B | n/a | error mapping (`writeErr`) unasserted |
| Web panel | C (type-check + build only) | B (`web/ARCHITECTURE.md`) | n/a | no component or E2E tests |
| Harness | B (negative tests run once, manual; evidence recorded by hand) | A | A | architecture script lacks its own regression test |

Grades are from a read-only audit on 2026-10-23, not measured coverage.
