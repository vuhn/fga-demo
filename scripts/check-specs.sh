#!/usr/bin/env bash
# Structure check for specs/<feature>/ (see specs/README.md). Content is judged by humans and the reviewer agents (docs/harness/review-rules.md).
set -u
cd "$(dirname "$0")/.."
fail=0
violation() { printf '\nFAIL: %s\n  why: %s\n  fix: %s\n' "$1" "$2" "$3"; fail=1; }

for d in specs/*/; do
  name=$(basename "$d")
  [ "$name" = "_templates" ] && continue
  if [ "$name" = "_adhoc" ]; then   # independent tasks: contracts only, no feature stages
    for f in $(find "$d" -type f); do   # specs/_adhoc/<yyyymmdd>/<slug>-contract.md
      [[ "$f" =~ ^specs/_adhoc/[0-9]{8}/[^/]+-contract\.md$ ]] || violation "$f is in specs/_adhoc/" "_adhoc holds only <yyyymmdd>/<slug>-contract.md" "move it to specs/_adhoc/<yyyymmdd>/<slug>-contract.md or into a feature"
    done
    continue
  fi
  [ -f "$d/README.md" ] || { violation "$d has no README.md" "the status table is how stages and gates are tracked" "copy specs/_templates/README.md"; continue; }
  for stage in system-design db-design plan release; do
    [ -f "$d/$stage.md" ] && ! [ -f "$d/requirements.md" ] && violation "$d$stage.md exists without requirements.md" "each stage reads the previous approved artifact" "write and approve requirements.md first"
  done
  [ -f "$d/plan.md" ] && grep -q "api/migrations" "$d/plan.md" && ! [ -f "$d/db-design.md" ] && grep -qi "skipped" "$d/README.md" \
    && grep -i "db design" "$d/README.md" | grep -qi skipped && violation "$d plans a migration but DB design is marked skipped" "schema changes need a db-design.md" "write db-design.md"
done
[ "$fail" -eq 0 ] && echo "specs: ok"
exit "$fail"
