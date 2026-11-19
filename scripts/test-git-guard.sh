#!/usr/bin/env bash
# Tests for hook-git-guard.sh: each case is "<expected exit>|<command the hook would see>".
cd "$(dirname "$0")/.."
fail=0
while IFS='|' read -r want cmd; do
  [ -z "$want" ] && continue
  got=$(python3 -c 'import json,sys;print(json.dumps({"tool_input":{"command":sys.argv[1]}}))' "$cmd" | bash scripts/hook-git-guard.sh >/dev/null 2>&1; echo $?)
  [ "$got" = "$want" ] || { echo "FAIL want $want got $got: $cmd"; fail=1; }
done <<'CASES'
2|git push origin main
2|git push origin HEAD:main
2|git push -u origin wt-t5
2|git push origin worktree-agent-abc123
2|git commit -m "x" -m "Co-Authored-By: Claude <noreply@anthropic.com>"
0|git commit -m "docs: update"
0|git push -u origin chore/harness-git-hygiene
0|git push origin HEAD:feat/re-arch-t4b
0|ls -la
CASES
[ $fail = 0 ] && echo "git guard: ok"
exit $fail
