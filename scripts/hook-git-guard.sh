#!/usr/bin/env bash
# Claude Code hook, PreToolUse (Bash). Enforces the git rules of CLAUDE.md:
#  1. no commit or push on main/master; 2. temp branches (wt-*, worktree-agent-*, working-tree-*) are never pushed;
#  3. commits carry the user as the only author: no Co-Authored-By / Claude attribution lines.
set -u
cmd=$(python3 -c 'import sys,json;print(json.load(sys.stdin).get("tool_input",{}).get("command",""))' 2>/dev/null)
case "$cmd" in *"git "*) ;; *) exit 0;; esac
block() { echo "Blocked: $1 (CLAUDE.md, Git rules)" >&2; exit 2; }
cur=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)
temp='(wt-[^ :]*|worktree-agent-[^ :]*|working-tree-[^ :]*)'
if printf '%s' "$cmd" | grep -qE 'git +(-C +[^ ]+ +)?commit'; then
  case "$cur" in main|master) block "do not commit on $cur; branch from the integration branch and open a PR";; esac
  printf '%s' "$cmd" | grep -qiE 'co-authored-by|generated with \[?claude|noreply@anthropic' && block "commits are authored by the user only: remove the Co-Authored-By / Claude line"
fi
if printf '%s' "$cmd" | grep -qE 'git +(-C +[^ ]+ +)?push'; then
  printf '%s' "$cmd" | grep -qE "(^|[ :/])(main|master)( |$|\")" && block "never push to main"
  printf '%s' "$cmd" | grep -qE "(^|[ :/])$temp( |$|\")" && block "temp branches stay local; push the commits to the task branch (git push origin HEAD:<task-branch>)"
  case "$cur" in main|master) block "never push from $cur";; wt-*|worktree-agent-*|working-tree-*)
    printf '%s' "$cmd" | grep -qE 'HEAD:[A-Za-z0-9._/-]+' || block "on a temp branch push only as: git push origin HEAD:<task-branch>";; esac
fi
exit 0
