#!/usr/bin/env bash
# PreToolUse / PostToolUse hook (Claude Code Edit|Write / Antigravity edit_file|write_file).
# PreToolUse: block edits to a migration that is already committed.
# PostToolUse: lint migrations after an edit. Reads the tool call JSON on stdin.
set -u
cd "$(dirname "$0")/.."
input=$(cat)
mode=$(printf '%s' "$input" | python3 -c 'import sys,json;d=json.load(sys.stdin);print("agy" if ("toolCall" in d or "hookEventName" in d) else "claude")' 2>/dev/null)
[ -z "$mode" ] && mode="claude"
if [ "$mode" = "agy" ]; then
  file=$(printf '%s' "$input" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("toolCall",{}).get("args",{}).get("Path",""))' 2>/dev/null)
  event=$(printf '%s' "$input" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("hookEventName",""))' 2>/dev/null)
else
  file=$(printf '%s' "$input" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("tool_input",{}).get("file_path",""))' 2>/dev/null)
  event=$(printf '%s' "$input" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("hook_event_name",""))' 2>/dev/null)
fi
allow() {
  if [ "$mode" = "agy" ]; then
    if [ "$event" = "PreToolUse" ]; then
      printf '{"decision":"allow"}\n'
    else
      printf '{}\n'
    fi
  fi
  exit 0
}
case "$file" in */api/migrations/*.sql|api/migrations/*.sql) ;; *) allow;; esac
rel=${file#"$PWD"/}
if [ "$event" = "PreToolUse" ]; then
  if git ls-files --error-unmatch "$rel" >/dev/null 2>&1; then
    msg="Blocked: $rel is already committed. Applied migrations are never edited; add a new migration (next number) instead. See .claude/skills/db-design."
    if [ "$mode" = "agy" ]; then
      python3 -c 'import json,sys;print(json.dumps({"decision":"deny","reason":sys.argv[1]}))' "$msg"
      exit 0
    fi
    echo "$msg" >&2
    exit 2
  fi
else
  out=$(bash scripts/check-migrations.sh 2>&1) || { printf '%s\n' "$out" >&2; exit 2; }
fi
allow
