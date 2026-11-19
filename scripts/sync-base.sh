#!/usr/bin/env bash
# Clock-in step 0: bring the integration branch (the branch a task's PR targets, e.g. feat/re-architecture
# or main) up to date with origin before anything else. Fast-forward only; never merges, never pushes.
#   scripts/sync-base.sh [branch]      (default: the branch checked out now)
# If you are on a task branch, pass its base: the script updates the local base ref without switching
# (git fetch origin base:base) and tells you whether the task branch is behind it.
set -eu
cd "$(dirname "$0")/.."
cur=$(git rev-parse --abbrev-ref HEAD)
base=${1:-$cur}
git fetch origin --prune --quiet
if ! git rev-parse --verify --quiet "origin/$base" >/dev/null; then
  echo "sync-base: origin/$base does not exist" >&2; exit 1
fi
if [ "$base" = "$cur" ]; then
  git merge --ff-only --quiet "origin/$base" || { echo "sync-base: $base has diverged from origin/$base; resolve by hand" >&2; exit 1; }
  echo "sync-base: $base is at $(git rev-parse --short HEAD) (origin/$base)"
else
  git fetch origin "$base:$base" --quiet 2>/dev/null || git branch -f "$base" "origin/$base" 2>/dev/null || true
  behind=$(git rev-list --count "HEAD..origin/$base")
  echo "sync-base: $base is at $(git rev-parse --short "origin/$base"); $cur is $behind commit(s) behind it"
  [ "$behind" = 0 ] || echo "sync-base: rebase or merge origin/$base into $cur before starting" >&2
fi
