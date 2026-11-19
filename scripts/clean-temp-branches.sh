#!/usr/bin/env bash
# Remove local agent worktrees and their temp branches (wt-*, worktree-agent-*, working-tree-*).
# Temp branches are local only: this never touches origin. Run after the task PR is created.
#   scripts/clean-temp-branches.sh           remove those whose work is on origin (or fully merged)
#   scripts/clean-temp-branches.sh --force   remove all of them, even with unpushed commits
set -eu
cd "$(dirname "$0")/.."
force=${1:-}
cur=$(git rev-parse --abbrev-ref HEAD)
git worktree prune
for b in $(git for-each-ref --format='%(refname:short)' 'refs/heads/wt-*' 'refs/heads/worktree-agent-*' 'refs/heads/working-tree-*'); do
  [ "$b" = "$cur" ] && continue
  if [ "$force" != "--force" ] && [ -n "$(git rev-list "$b" --not --remotes 2>/dev/null | head -1)" ]; then
    echo "keep $b: has commits that are on no remote branch (push them to the task branch or use --force)"; continue
  fi
  wt=$(git worktree list --porcelain | awk -v b="refs/heads/$b" '/^worktree /{p=$2} $0=="branch " b{print p}')
  [ -n "$wt" ] && git worktree remove --force "$wt"
  git branch -D "$b" >/dev/null && echo "removed $b"
done
