#!/usr/bin/env bash
# HTTP replay of the blueprint and estimate routes. Records, per step, the HTTP status and the JSON *shape*
# (sorted key paths, no values that change between runs) so the same script run before and after the
# re-architecture can be diffed. Needs the stack running: compose, seeded data, `make api`, and the engine
# (legacy `make engine` or, after T9, the Engine gRPC server). LLM steps run only when OLLAMA=1.
#
#   scripts/smoke-replay.sh [outfile]      default: /dev/stdout
#   OLLAMA=1 scripts/smoke-replay.sh out.txt
# Creates and deletes one throwaway blueprint in Acme's Annex project (Cara, internal admin).
set -u
cd "$(dirname "$0")/.."
API=${API:-http://localhost:8088/api/v1}
OUT=${1:-/dev/stdout}
SAMPLES=scripts/smoke-fixtures
: >"$OUT" 2>/dev/null || true
exec 3>>"$OUT"
fail=0
BODYF=$(mktemp); trap 'rm -f "$BODYF"' EXIT

login() { curl -s -XPOST "$API/auth/login" -H 'content-type: application/json' -d "{\"email\":\"$1@example.com\",\"password\":\"password\"}" | jq -r .access_token; }
CARA=$(login cara); DAN=$(login dan)
[ -n "$CARA" ] && [ "$CARA" != null ] || { echo "login failed: is the API up and seeded?" >&2; exit 2; }
TENANT=$(curl -s "$API/tenants/mine" -H "authorization: Bearer $CARA" | jq -r '[.[]|select(.name=="Acme")][0].id')
PROJECT=a1a1a1a1-0000-4000-8000-000000000002   # Annex: Cara is internal admin

# shape: sorted key paths, arrays collapsed to [], scalars to their JSON type
shape() { jq -r '[paths(scalars)|map(if type=="number" then "[]" else . end)|join(".")]|unique|.[]' 2>/dev/null | sed 's/\.\[\]/[]/g'; }

step() { # name token method path [curl args...]; the response body is in $BODY, status in $CODE
  local name=$1 tok=$2 method=$3 path=$4; shift 4
  local resp; resp=$(curl -s -o "$BODYF" -w '%{http_code}' -X "$method" "$API$path" -H "authorization: Bearer $tok" -H "X-Tenant-ID: $TENANT" "$@")
  CODE=$resp; BODY=$(cat "$BODYF")
  { echo "## $name  ->  $CODE"; if jq -e . "$BODYF" >/dev/null 2>&1; then shape <"$BODYF" | sed 's/^/   /'; else echo "   (non-JSON body)"; fi; } >&3
}

J=(-H 'content-type: application/json')
# --- no LLM -----------------------------------------------------------------------------------------------
step "import preview dxf"  "$CARA" POST /blueprint-drafts/dxf-import -F "project_id=$PROJECT" -F "file=@$SAMPLES/family_3br.dxf" -F unit=m
step "import preview pdf (longest_m)" "$CARA" POST /blueprint-drafts/dxf-import -F "project_id=$PROJECT" -F "file=@$SAMPLES/studio.pdf" -F longest_m=12
step "import preview png (longest_m)" "$CARA" POST /blueprint-drafts/dxf-import -F "project_id=$PROJECT" -F "file=@$SAMPLES/studio.png" -F longest_m=8
step "import preview non-cad file" "$CARA" POST /blueprint-drafts/dxf-import -F "project_id=$PROJECT" -F "file=@README.md;filename=x.dxf"
step "import preview dan (external)" "$DAN" POST /blueprint-drafts/dxf-import -F "project_id=$PROJECT" -F "file=@$SAMPLES/family_3br.dxf"
step "import" "$CARA" POST /blueprints/import -F "project_id=$PROJECT" -F "name=smoke-import" -F "file=@$SAMPLES/family_3br.dxf" -F unit=m
IMPORTED=$(echo "$BODY" | jq -r '.blueprint.id // .id // empty')
[ -n "$IMPORTED" ] || { echo "import failed, stopping" >&2; fail=1; }
if [ -n "$IMPORTED" ]; then
  step "get" "$CARA" GET "/blueprints/$IMPORTED"; V=$(echo "$BODY" | jq -r '.version.version // .current_version // 1')
  step "get as dan (404)" "$DAN" GET "/blueprints/$IMPORTED"
  step "list" "$CARA" GET "/blueprints?project_id=$PROJECT"
  step "list as dan" "$DAN" GET "/blueprints?project_id=$PROJECT"
  step "versions" "$CARA" GET "/blueprints/$IMPORTED/versions"
  step "drawing svg" "$CARA" GET "/blueprints/$IMPORTED/drawing?format=svg"
  step "drawing pdf" "$CARA" GET "/blueprints/$IMPORTED/drawing?format=pdf"
  step "drawing dxf" "$CARA" GET "/blueprints/$IMPORTED/drawing?format=dxf"
  step "drawing ifc" "$CARA" GET "/blueprints/$IMPORTED/drawing?format=ifc"
  step "source (original)" "$CARA" GET "/blueprints/$IMPORTED/source"
  step "source as dan (403/404)" "$DAN" GET "/blueprints/$IMPORTED/source"
  step "plan-edit rename (no change in base_version)" "$CARA" POST "/blueprints/$IMPORTED/plan-edits" "${J[@]}" -d "{\"base_version\":$V,\"ops\":{\"ops\":[{\"op\":\"rename_room\",\"room\":\"$(curl -s "$API/blueprints/$IMPORTED" -H "authorization: Bearer $CARA" -H "X-Tenant-ID: $TENANT" | jq -r '.version.model.rooms[0].id')\",\"name\":\"Smoke room\"}]}}"
  V2=$((V+1))
  step "plan-edit stale base_version (409)" "$CARA" POST "/blueprints/$IMPORTED/plan-edits" "${J[@]}" -d '{"base_version":1,"ops":{"ops":[]}}'
  step "plan-edit bad op (400)" "$CARA" POST "/blueprints/$IMPORTED/plan-edits" "${J[@]}" -d "{\"base_version\":$V2,\"ops\":{\"ops\":[{\"op\":\"remove_opening\",\"opening\":\"nope\"}]}}"
  step "export edited dxf" "$CARA" GET "/blueprints/$IMPORTED/source/edited"
  step "rename" "$CARA" PATCH "/blueprints/$IMPORTED" "${J[@]}" -d '{"name":"smoke-import-2"}'
  step "share candidates" "$CARA" GET "/blueprints/$IMPORTED/share-candidates"
  step "review on a DXF plan (400)" "$CARA" POST "/blueprints/$IMPORTED/reviews" "${J[@]}" -d "{\"base_version\":$V2}"
  step "estimates list" "$CARA" GET "/blueprints/$IMPORTED/estimates"
  step "estimate session (none: 404)" "$CARA" GET "/blueprints/$IMPORTED/estimate-session"
  step "estimate session create" "$CARA" POST "/blueprints/$IMPORTED/estimate-session"
  step "estimate session create as dan (403)" "$DAN" POST "/blueprints/$IMPORTED/estimate-session"
fi
BRIEF='{"title":"Garden studio","lot":{"width":12,"depth":15},"entrance_side":"east","rooms":[{"id":"living","name":"Studio","type":"living","target_area":20},{"id":"kitchen","name":"Kitchenette","type":"kitchen","target_area":7},{"id":"bath","name":"Shower room","type":"bathroom","target_area":3.5}]}'
step "create from brief (form)" "$CARA" POST /blueprints "${J[@]}" -d "{\"project_id\":\"$PROJECT\",\"name\":\"smoke-create\",\"brief\":$BRIEF,\"source_kind\":\"form\"}"
CREATED=$(echo "$BODY" | jq -r '.blueprint.id // .id // empty')
step "create with invalid brief (400)" "$CARA" POST /blueprints "${J[@]}" -d "{\"project_id\":\"$PROJECT\",\"name\":\"x\",\"brief\":{},\"source_kind\":\"form\"}"
step "create as dan (403)" "$DAN" POST /blueprints "${J[@]}" -d "{\"project_id\":\"$PROJECT\",\"name\":\"x\",\"brief\":$BRIEF,\"source_kind\":\"form\"}"
# --- LLM (needs Ollama) -----------------------------------------------------------------------------------
if [ "${OLLAMA:-}" = 1 ] && [ -n "$CREATED" ]; then
  step "draft from text" "$CARA" POST /blueprint-drafts "${J[@]}" -d "{\"project_id\":\"$PROJECT\",\"prompt\":\"A 2 bedroom house on a 12 by 18 m lot, entrance on the south\"}"
  step "revise (chat)" "$CARA" POST "/blueprints/$CREATED/revisions" "${J[@]}" -d '{"base_version":1,"message":"rename the first bedroom to Study"}'
  step "revise stale (409)" "$CARA" POST "/blueprints/$CREATED/revisions" "${J[@]}" -d '{"base_version":1,"message":"x"}'
  step "revise as dan (403)" "$DAN" POST "/blueprints/$CREATED/revisions" "${J[@]}" -d '{"base_version":1,"message":"x"}'
  step "estimate session create" "$CARA" POST "/blueprints/$CREATED/estimate-session"
  step "estimate turn (SSE)" "$CARA" POST "/blueprints/$CREATED/estimate-session/messages" "${J[@]}" -d '{"message":"This is an apartment in Ho Chi Minh City, normal quality"}' --max-time 600
fi
for id in $IMPORTED $CREATED; do [ -n "$id" ] && step "delete" "$CARA" DELETE "/blueprints/$id"; done
exit $fail
