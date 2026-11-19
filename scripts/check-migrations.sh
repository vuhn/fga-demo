#!/usr/bin/env bash
# Backward-compatibility lint for goose migrations (see .claude/skills/db-design).
# Only migrations numbered above BASELINE are checked; older ones are history.
# A line may opt out with `-- breaking-ok: <reason>` (the reason belongs in specs/<feature>/db-design.md).
set -u
cd "$(dirname "$0")/.."

BASELINE=11
fail=0
violation() { printf '\nFAIL: %s\n  why: %s\n  fix: %s\n' "$1" "$2" "$3"; fail=1; }

for f in api/migrations/[0-9]*.sql; do
  n=$((10#$(basename "$f" | cut -d_ -f1)))
  [ "$n" -le "$BASELINE" ] && continue
  # Only the Up section matters for rollout safety.
  up=$(awk '/\+goose Down/{exit} {print NR": "$0}' "$f")
  while IFS= read -r line; do
    [ -z "$line" ] && continue
    case "$line" in *"breaking-ok:"*) continue;; esac
    code=$(printf '%s' "$line" | sed 's/--.*//' | tr 'a-z' 'A-Z')
    why="" fix=""
    case "$code" in
      *"DROP COLUMN"*|*"DROP TABLE"*) why="dropping something old code may still read breaks the running release"; fix="expand first, drop in a later release once nothing reads it";;
      *"RENAME "*) why="renaming breaks old code during rollout"; fix="add the new name, backfill, drop the old one in a later release";;
      *"SET NOT NULL"*) why="existing or in-flight rows may be NULL"; fix="backfill first and add a CHECK ... NOT VALID, or keep it nullable";;
      *"DROP NOT NULL"*) why="NOT NULL -> NULL breaks readers that assume a value"; fix="keep NOT NULL, or add a new column";;
      *"ALTER TYPE"*|*"ADD VALUE"*|*"DROP TYPE"*) why="changing enum/type values breaks old code"; fix="use a new nullable column or lookup table";;
      *"ADD COLUMN"*"NOT NULL"*) case "$code" in *DEFAULT*) ;; *) why="new NOT NULL column without DEFAULT breaks inserts from old code"; fix="add a DEFAULT, or add nullable then backfill";; esac;;
    esac
    [ -n "$why" ] && violation "$f: $line" "$why" "$fix, or mark the line '-- breaking-ok: <reason>' and justify it in db-design.md"
  done <<< "$up"
  grep -q '+goose Down' "$f" || violation "$f has no '-- +goose Down' section" "migrations must be reversible or say why not" "add a Down section (an explicit no-op with a comment if irreversible)"
done

[ "$fail" -eq 0 ] && echo "migrations: ok"
exit "$fail"
