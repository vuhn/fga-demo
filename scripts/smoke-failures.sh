#!/usr/bin/env bash
# Failure matrix: start a second API on :8089 whose Engine or Agent address points at a dead port and assert which
# routes answer 502 `engine_unavailable` and which keep working. Non-destructive (the running stack is not
# touched); the second API shares the database, and a throwaway blueprint is created and deleted.
#
#   scripts/smoke-failures.sh engine-down      # the API's Engine address is dead: deterministic routes 502 (LLM routes skipped: the Agent has its own Engine)
#   scripts/smoke-failures.sh agent-down       # LLM routes 502, deterministic routes 200
# Needs the normal stack (compose, seed, a first API on :8088 and a live engine) to create the fixture.
set -u
cd "$(dirname "$0")/.."
MODE=${1:-engine-down}
API1=${API1:-http://localhost:8088/api/v1}
PORT=8089
API2=http://localhost:$PORT/api/v1
FIX=scripts/smoke-fixtures
PROJECT=a1a1a1a1-0000-4000-8000-000000000002
bad=0

login() { curl -s -XPOST "$API1/auth/login" -H 'content-type: application/json' -d '{"email":"cara@example.com","password":"password"}' | jq -r .access_token; }
TOK=$(login); [ -n "$TOK" ] && [ "$TOK" != null ] || { echo "first API not reachable on :8088" >&2; exit 2; }
TENANT=$(curl -s "$API1/tenants/mine" -H "authorization: Bearer $TOK" | jq -r '[.[]|select(.name=="Acme")][0].id')

case $MODE in
  agent-down)  DEAD=(AGENT_ADDR=127.0.0.1:1) ;;
  engine-down) DEAD=(BLUEPRINT_ENGINE_ADDR=127.0.0.1:1) ;;
  *) echo "unknown mode $MODE" >&2; exit 2 ;;
esac

# Fixture through the healthy API.
ID=$(curl -s -XPOST "$API1/blueprints/import" -H "authorization: Bearer $TOK" -H "X-Tenant-ID: $TENANT" \
  -F "project_id=$PROJECT" -F name=smoke-failures -F "file=@$FIX/studio.dxf" -F unit=m | jq -r '.blueprint.id // .id // empty')
[ -n "$ID" ] || { echo "fixture import failed" >&2; exit 2; }
trap 'curl -s -o /dev/null -XDELETE "$API1/blueprints/$ID" -H "authorization: Bearer $TOK" -H "X-Tenant-ID: $TENANT"; kill ${PID:-0} 2>/dev/null' EXIT
curl -s -o /dev/null -XPOST "$API1/blueprints/$ID/estimate-session" -H "authorization: Bearer $TOK" -H "X-Tenant-ID: $TENANT"

(cd api && env HTTP_ADDR=:$PORT "${DEAD[@]}" go run ./cmd/api >/tmp/smoke-failures-api.log 2>&1) &
PID=$!
for _ in $(seq 1 60); do curl -s -o /dev/null "$API2/auth/login" -XPOST && break; sleep 1; done

expect() { # want name method path [curl args]
  local want=$1 name=$2 method=$3 path=$4; shift 4
  local code; code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 60 -X "$method" "$API2$path" -H "authorization: Bearer $TOK" -H "X-Tenant-ID: $TENANT" "$@")
  if [ "$code" = "$want" ] || { [ "$want" = 200 ] && [ "$code" = 201 ]; }; then echo "ok    $name -> $code"; else echo "FAIL  $name -> $code (want $want)"; bad=1; fi
}
J=(-H 'content-type: application/json')
V=$(curl -s "$API1/blueprints/$ID" -H "authorization: Bearer $TOK" -H "X-Tenant-ID: $TENANT" | jq -r '.version.version // 1')
ROOM=$(curl -s "$API1/blueprints/$ID" -H "authorization: Bearer $TOK" -H "X-Tenant-ID: $TENANT" | jq -r '.version.model.rooms[0].id')

# Always unaffected by a dead Engine or Agent: reads of stored data and the restore.
expect 200 "get (stored)"            GET "/blueprints/$ID"
expect 200 "list"                    GET "/blueprints?project_id=$PROJECT"
expect 200 "versions"                GET "/blueprints/$ID/versions"
expect 200 "source (stored original)" GET "/blueprints/$ID/source"
expect 200 "rename"                  PATCH "/blueprints/$ID" "${J[@]}" -d '{"name":"smoke-failures"}'

DET=502; LLM=502          # deterministic and LLM routes
[ "$MODE" = agent-down ] && DET=200
expect $DET "import preview"  POST /blueprint-drafts/dxf-import -F "project_id=$PROJECT" -F "file=@$FIX/studio.dxf" -F unit=m
expect $DET "create from brief" POST /blueprints "${J[@]}" -d "{\"project_id\":\"$PROJECT\",\"name\":\"x\",\"source_kind\":\"form\",\"brief\":{\"title\":\"s\",\"lot\":{\"width\":12,\"depth\":15},\"entrance_side\":\"east\",\"rooms\":[{\"id\":\"living\",\"name\":\"Studio\",\"type\":\"living\",\"target_area\":20},{\"id\":\"kitchen\",\"name\":\"K\",\"type\":\"kitchen\",\"target_area\":7},{\"id\":\"bath\",\"name\":\"B\",\"type\":\"bathroom\",\"target_area\":3.5}]}}"
expect $DET "drawing svg (render)" GET "/blueprints/$ID/drawing?format=svg"
expect $DET "export edited dxf" GET "/blueprints/$ID/source/edited"
expect $DET "plan-edits" POST "/blueprints/$ID/plan-edits" "${J[@]}" -d "{\"base_version\":$V,\"ops\":{\"ops\":[{\"op\":\"rename_room\",\"room\":\"$ROOM\",\"name\":\"X\"}]}}"
V=$(curl -s "$API1/blueprints/$ID" -H "authorization: Bearer $TOK" -H "X-Tenant-ID: $TENANT" | jq -r '.version.version // 1')   # plan-edits above made a new version
# engine-down kills only the API's own Engine address: the live Agent still reaches its Engine, so the LLM routes are not
# expected to fail there (and would spend real LLM time); they are asserted in agent-down.
if [ "$MODE" = agent-down ]; then
  expect $LLM "draft from text" POST /blueprint-drafts "${J[@]}" -d "{\"project_id\":\"$PROJECT\",\"prompt\":\"a small studio\"}"
  expect $LLM "revise (chat)"   POST "/blueprints/$ID/revisions" "${J[@]}" -d "{\"base_version\":$V,\"message\":\"rename a room\"}"
  expect $LLM "revise stream (refused before the stream opens)" POST "/blueprints/$ID/revisions/stream" "${J[@]}" -d "{\"base_version\":$V,\"message\":\"rename a room\"}"
  expect $LLM "estimate turn"   POST "/blueprints/$ID/estimate-session/messages" "${J[@]}" -d '{"message":"an apartment in Hanoi"}'
fi
echo "mode $MODE: $([ $bad = 0 ] && echo PASS || echo FAIL)"
exit $bad
