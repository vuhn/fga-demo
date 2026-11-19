# Lessons

Newest first. One entry per incident or repeated agent mistake: symptom, cause, prevention (rule, check, or test added).

- 2026-11-16: `make seed` failed on foreign keys after new tables were added. Cause: `Truncate` did not list them. Prevention: `db-design` skill checklist requires updating `Truncate`.
