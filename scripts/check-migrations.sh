#!/usr/bin/env bash
# Backward-compatibility lint for goose migrations (see .claude/skills/db-design); the rules are in check-migrations.py.
# Only migrations numbered above BASELINE are checked. A statement opts out with
# `-- force-allow-breaking: <reason>` (the reason belongs in specs/<feature>/db-design.md).
cd "$(dirname "$0")/.." && exec python3 scripts/check-migrations.py "$@"
