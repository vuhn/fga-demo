#!/usr/bin/env bash
# PreToolUse hook (Claude Code Bash / Antigravity run_command). Enforces the git rules of CLAUDE.md:
#  1. no commit or push on main/master; 2. temp branches (wt-*, worktree-agent-*, working-tree-*) are never pushed;
#  3. commits carry the user as the only author: no Co-Authored-By / agent attribution lines.
set -u
cd "$(dirname "$0")/.."
input=$(cat)
mode=$(printf '%s' "$input" | python3 -c 'import sys,json;d=json.load(sys.stdin);print("agy" if ("toolCall" in d or "hookEventName" in d) else "claude")' 2>/dev/null)
[ -z "$mode" ] && mode="claude"
if [ "$mode" = "agy" ]; then
  cmd=$(printf '%s' "$input" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("toolCall",{}).get("args",{}).get("CommandLine",""))' 2>/dev/null)
else
  cmd=$(printf '%s' "$input" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("tool_input",{}).get("command",""))' 2>/dev/null)
fi
allow() {
  [ "$mode" = "agy" ] && printf '{"decision":"ask"}\n'
  exit 0
}
block() {
  msg="Blocked: $1 (CLAUDE.md, Git rules)"
  if [ "$mode" = "agy" ]; then
    python3 -c 'import json,sys;print(json.dumps({"decision":"deny","reason":sys.argv[1]}))' "$msg"
    exit 0
  fi
  echo "$msg" >&2
  exit 2
}
case "$cmd" in *"git "*) ;; *) allow;; esac
cur=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)
temp='(wt-[^ :]*|worktree-agent-[^ :]*|working-tree-[^ :]*)'
if printf '%s' "$cmd" | grep -qE 'git +(-C +[^ ]+ +)?commit'; then
  case "$cur" in main|master) block "do not commit on $cur; branch from the integration branch and open a PR";; esac
  printf '%s' "$cmd" | grep -qiE 'co-authored-by|generated with \[?(claude|antigravity)|noreply@anthropic' && block "commits are authored by the user only: remove the Co-Authored-By / attribution line"
fi
if printf '%s' "$cmd" | grep -qE 'git +(-C +[^ ]+ +)?push'; then
  printf '%s' "$cmd" | grep -qE "(^|[ :/])(main|master)( |$|\")" && block "never push to main"
  printf '%s' "$cmd" | grep -qE "(^|[ :/])$temp( |$|\")" && block "temp branches stay local; push the commits to the task branch (git push origin HEAD:<task-branch>)"
  case "$cur" in main|master) block "never push from $cur";; wt-*|worktree-agent-*|working-tree-*)
    printf '%s' "$cmd" | grep -qE 'HEAD:[A-Za-z0-9._/-]+' || block "on a temp branch push only as: git push origin HEAD:<task-branch>";; esac
fi
allow
