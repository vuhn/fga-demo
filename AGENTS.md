# AGENTS.md

Single source of agent instructions: [CLAUDE.md](CLAUDE.md). Read it first; do not copy its rules here.

- Verify with `make check` (static) and `make verify-live` (needs Docker; resets data).
- Session state: `docs/harness/progress.md`.
- Features follow the AI-native SDLC: `/feature <name>`, artifacts in `specs/`, map in `docs/harness/sdlc.md`.
- Shared skills & hooks: `.agents/skills.json` exposes `.claude/skills/` and `.agents/hooks.json` runs the shared scripts in `scripts/hook-*.sh`. Follow every skill's invocation guard (never start `feature` unless the user invokes it explicitly).
- Reviewer subagents: `.claude/agents/*.md` is the single source of truth. In runtimes that do not auto-register `.claude/agents/`, read `.claude/agents/<name>.md` and launch it as a read-only subagent (`EnableWriteTools: false`).
