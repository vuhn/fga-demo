# Diagrams

Standalone HTML diagrams made with Archify (open in a browser, no server needed). They explain the `/feature` skill and cite `.claude/skills/` and `specs/README.md` with line numbers pinned to commit `07363ef`; the lines drift as those files change, so regenerate rather than hand-edit.

| File | Shows |
| --- | --- |
| `feature-skill-gates.html` | `/feature` stage chain 1-7, with an approval gate after every stage and the stage reviewer (`system-design-reviewer`, `db-design-reviewer`, `plan-reviewer`) before gates 2-4 |
| `build-task-detail.html` | `build-task` in five phases: contract gate, implement, review (max 2 rounds), stop and retry paths |
