#!/usr/bin/env bash
# Claude Code hook. PreToolUse (Edit|Write): block edits to a migration that is already committed.
# PostToolUse: lint migrations after an edit. Reads the tool call JSON on stdin.
set -u
cd "$(dirname "$0")/.."
input=$(cat)
file=$(printf '%s' "$input" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("tool_input",{}).get("file_path",""))' 2>/dev/null)
event=$(printf '%s' "$input" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("hook_event_name",""))' 2>/dev/null)
case "$file" in */api/migrations/*.sql) ;; *) exit 0;; esac
rel=${file#"$PWD"/}
if [ "$event" = "PreToolUse" ]; then
  if git ls-files --error-unmatch "$rel" >/dev/null 2>&1; then
    echo "Blocked: $rel is already committed. Applied migrations are never edited; add a new migration (next number) instead. See .claude/skills/db-design." >&2
    exit 2
  fi
else
  out=$(bash scripts/check-migrations.sh 2>&1) || { printf '%s\n' "$out" >&2; exit 2; }
fi
exit 0
