#!/usr/bin/env bash
# HTTP replay of every non-blueprint, non-estimate domain: users, auth/me, tenants, companies, products, members,
# roles, inspector, permissions, folders, albums, documents, photos, projects, project members and project roles.
# Records, per step, the HTTP status and the JSON *shape* (sorted key paths, no values that change between runs)
# so a run before and after a refactor can be diffed (see specs/usecase-io-structs). Needs the stack running:
# compose, seeded data, `make api`. No LLM, no engine, no Agent.
#
#   scripts/smoke-replay-domains.sh [outfile]      default: /dev/stdout
# Creates and deletes its own throwaway rows (a folder tree, an album, a project, roles, a member). It also
# creates one user, one company, one project and one tenant per run (there is no delete route); a reseed removes them.
set -u
cd "$(dirname "$0")/.."
API=${API:-http://localhost:8088/api/v1}
OUT=${1:-/dev/stdout}
: >"$OUT" 2>/dev/null || true
exec 3>>"$OUT"
BODYF=$(mktemp); trap 'rm -f "$BODYF"' EXIT
RUN=$(date +%s)$RANDOM

login() { curl -s -XPOST "$API/auth/login" -H 'content-type: application/json' -d "{\"email\":\"$1@example.com\",\"password\":\"password\"}" | jq -r .access_token; }
ADA=$(login ada); BOB=$(login bob); CARA=$(login cara); DAN=$(login dan); EVE=$(login eve)
[ -n "$BOB" ] && [ "$BOB" != null ] || { echo "login failed: is the API up and seeded?" >&2; exit 2; }
ACME=aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa; GLOBEX=bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb
TOWER=a1a1a1a1-0000-4000-8000-000000000001
BOBID=22222222-2222-2222-2222-222222222222; DANID=44444444-4444-4444-4444-444444444444
TENANT=$ACME

shape() { jq -r '[paths(scalars)|map(if type=="number" then "[]" else . end)|join(".")]|unique|.[]' 2>/dev/null | sed 's/\.\[\]/[]/g'; }

step() { # name token method path [curl args...]; the response body is in $BODY, status in $CODE
  local name=$1 tok=$2 method=$3 path=$4; shift 4
  CODE=$(curl -s -o "$BODYF" -w '%{http_code}' -X "$method" "$API$path" -H "authorization: Bearer $tok" -H "X-Tenant-ID: $TENANT" "$@")
  BODY=$(cat "$BODYF")
  { echo "## $name  ->  $CODE"; if jq -e . "$BODYF" >/dev/null 2>&1; then shape <"$BODYF" | sed 's/^/   /'; else echo "   (non-JSON body)"; fi; } >&3
}
J=(-H 'content-type: application/json')
id() { echo "$BODY" | jq -r "$1 // empty"; }

# --- auth, me, permissions, access -------------------------------------------------------------------------
step "login ok" "" POST /auth/login "${J[@]}" -d '{"email":"bob@example.com","password":"password"}'
step "login wrong password (401)" "" POST /auth/login "${J[@]}" -d '{"email":"bob@example.com","password":"nope"}'
step "me bob" "$BOB" GET /me
step "me without token (401)" "" GET /me
step "my permissions bob platform" "$BOB" GET /me/permissions
step "my permissions bob acme" "$BOB" GET "/me/permissions?tenant_id=$ACME"
step "my permissions dan acme" "$DAN" GET "/me/permissions?tenant_id=$ACME"
step "my permissions ada" "$ADA" GET /me/permissions
step "inspector check allow" "$BOB" POST /authz/check "${J[@]}" -d "{\"user_id\":\"$BOBID\",\"relation\":\"can_view\",\"object\":\"folder:f1111111-1111-1111-1111-111111111111\"}"
step "inspector check dan" "$DAN" POST /authz/check "${J[@]}" -d "{\"user_id\":\"$DANID\",\"relation\":\"can_view\",\"object\":\"folder:f1111111-1111-1111-1111-111111111111\"}"
step "inspector batch" "$BOB" POST /authz/batch-check "${J[@]}" -d "{\"checks\":[{\"user_id\":\"$BOBID\",\"relation\":\"can_view\",\"object\":\"folder:f1111111-1111-1111-1111-111111111111\"}]}"
step "access report bob" "$BOB" GET "/access?object_type=folder&object_id=f1111111-1111-1111-1111-111111111111"
step "access report dan (403/404)" "$DAN" GET "/access?object_type=folder&object_id=f1111111-1111-1111-1111-111111111111"

# --- users, tenants, companies, products --------------------------------------------------------------------
step "users list ada" "$ADA" GET /users
step "users list bob (403)" "$BOB" GET /users
step "user create ada" "$ADA" POST /users "${J[@]}" -d "{\"email\":\"replay$RUN@example.com\",\"name\":\"Replay\",\"password\":\"password\",\"company_id\":\"90000000-0000-4000-8000-000000000001\"}"
NEWUSER=$(id .id)
step "user create bob (403)" "$BOB" POST /users "${J[@]}" -d "{\"email\":\"x$RUN@example.com\",\"name\":\"X\",\"password\":\"password\"}"
[ -n "$NEWUSER" ] && step "user rename" "$ADA" PATCH "/users/$NEWUSER" "${J[@]}" -d '{"name":"Replay Renamed"}'
step "tenants list ada" "$ADA" GET /tenants
step "tenants list bob (403)" "$BOB" GET /tenants
step "tenants mine bob" "$BOB" GET /tenants/mine
step "tenant get acme" "$BOB" GET "/tenants/$ACME"
step "tenant get globex as bob (403/404)" "$BOB" GET "/tenants/$GLOBEX"
step "tenant create bob (403)" "$BOB" POST /tenants "${J[@]}" -d "{\"name\":\"Replay$RUN\",\"slug\":\"replay-$RUN\"}"
step "tenant create ada" "$ADA" POST /tenants "${J[@]}" -d "{\"name\":\"Replay$RUN\",\"slug\":\"replay-$RUN\"}"
NEWTENANT=$(id .id)
[ -n "$NEWTENANT" ] && step "tenant rename" "$ADA" PATCH "/tenants/$NEWTENANT" "${J[@]}" -d '{"name":"Replay renamed"}'
step "companies all" "$ADA" GET /companies
step "companies of acme" "$BOB" GET "/tenants/$ACME/companies"
step "company create" "$BOB" POST "/tenants/$ACME/companies" "${J[@]}" -d "{\"name\":\"Replay Co $RUN\"}"
step "company create dan (403)" "$DAN" POST "/tenants/$ACME/companies" "${J[@]}" -d '{"name":"x","tax_id":"x"}'
step "products bob" "$BOB" GET "/tenants/$ACME/products"
step "products dan (403)" "$DAN" GET "/tenants/$ACME/products"
step "product toggle bob (403)" "$BOB" PUT "/tenants/$ACME/products/blueprint" "${J[@]}" -d '{"enabled":true}'
step "product toggle ada" "$ADA" PUT "/tenants/$ACME/products/blueprint" "${J[@]}" -d '{"enabled":true}'

# --- members -------------------------------------------------------------------------------------------------
step "members list bob" "$BOB" GET "/tenants/$ACME/members"
step "members list dan (403)" "$DAN" GET "/tenants/$ACME/members"
step "member add (new user)" "$BOB" POST "/tenants/$ACME/members" "${J[@]}" -d "{\"email\":\"member$RUN@example.com\",\"name\":\"Member\",\"password\":\"password\",\"company_id\":\"90000000-0000-4000-8000-000000000004\",\"role\":\"external_user\"}"
MEMBER=$(id .user_id)
[ -n "$MEMBER" ] || MEMBER=$(id .user.id)
if [ -n "$MEMBER" ]; then
  step "member change role" "$BOB" PATCH "/tenants/$ACME/members/$MEMBER" "${J[@]}" -d '{"role":"external_admin"}'
  step "member change role dan (403)" "$DAN" PATCH "/tenants/$ACME/members/$MEMBER" "${J[@]}" -d '{"role":"external_user"}'
  step "member remove" "$BOB" DELETE "/tenants/$ACME/members/$MEMBER"
fi

# --- custom roles (tenant) ------------------------------------------------------------------------------------
step "roles list" "$BOB" GET "/tenants/$ACME/roles"
step "role create (platform only: bob 403)" "$BOB" POST "/tenants/$ACME/roles" "${J[@]}" -d "{\"name\":\"Replay $RUN\",\"affiliation\":\"internal\"}"
step "role create ada" "$ADA" POST "/tenants/$ACME/roles" "${J[@]}" -d "{\"name\":\"Replay $RUN\",\"affiliation\":\"internal\"}"
ROLE=$(id .id)
if [ -n "$ROLE" ]; then
  step "role get" "$BOB" GET "/tenants/$ACME/roles/$ROLE"
  step "role rename" "$ADA" PATCH "/tenants/$ACME/roles/$ROLE" "${J[@]}" -d '{"name":"Replay renamed"}'
  step "role set permissions" "$ADA" PUT "/tenants/$ACME/roles/$ROLE/permissions" "${J[@]}" -d '{"permissions":["view","edit"]}'
  step "role set permissions (bad name 400)" "$ADA" PUT "/tenants/$ACME/roles/$ROLE/permissions" "${J[@]}" -d '{"permissions":["nope"]}'
  step "role delete" "$ADA" DELETE "/tenants/$ACME/roles/$ROLE"
fi

# --- projects, project members, project roles -------------------------------------------------------------------
step "projects list bob" "$BOB" GET /projects
step "projects list dan" "$DAN" GET /projects
step "project get tower" "$BOB" GET "/projects/$TOWER"
step "project get as eve (403/404)" "$EVE" GET "/projects/$TOWER"
step "project create" "$BOB" POST /projects "${J[@]}" -d "{\"name\":\"Replay $RUN\"}"
PROJ=$(id .id)
step "project create dan (403)" "$DAN" POST /projects "${J[@]}" -d '{"name":"x"}'
if [ -n "$PROJ" ]; then
  step "project rename" "$BOB" PATCH "/projects/$PROJ" "${J[@]}" -d "{\"name\":\"Replay renamed $RUN\"}"
  step "project member candidates" "$BOB" GET "/projects/$PROJ/member-candidates"
  step "project member add dan" "$BOB" POST "/projects/$PROJ/members" "${J[@]}" -d "{\"user_id\":\"$DANID\",\"role\":\"external_user\"}"
  step "project members list" "$BOB" GET "/projects/$PROJ/members"
  step "project member change role" "$BOB" PATCH "/projects/$PROJ/members/$DANID" "${J[@]}" -d '{"role":"external_admin"}'
  step "project member add as dan (403)" "$DAN" POST "/projects/$PROJ/members" "${J[@]}" -d "{\"user_id\":\"$DANID\",\"role\":\"external_user\"}"
  step "project roles list" "$BOB" GET "/projects/$PROJ/roles"
  step "project role create" "$ADA" POST "/projects/$PROJ/roles" "${J[@]}" -d "{\"name\":\"Replay $RUN\",\"affiliation\":\"external\"}"
  PROLE=$(id .id)
  if [ -n "$PROLE" ]; then
    step "project role set permissions" "$ADA" PUT "/projects/$PROJ/roles/$PROLE/permissions" "${J[@]}" -d '{"permissions":["view"]}'
    step "project role delete" "$ADA" DELETE "/projects/$PROJ/roles/$PROLE"
  fi
  step "project member remove" "$BOB" DELETE "/projects/$PROJ/members/$DANID"
fi

# --- folders, documents ---------------------------------------------------------------------------------------
step "folders list bob" "$BOB" GET /folders
step "folders list dan" "$DAN" GET "/folders?project_id=$TOWER"
step "folder get" "$BOB" GET /folders/f1111111-1111-1111-1111-111111111111
step "folder get as dan (404)" "$DAN" GET /folders/f1111111-1111-1111-1111-111111111111
step "folder create" "$BOB" POST /folders "${J[@]}" -d "{\"project_id\":\"$TOWER\",\"name\":\"Replay $RUN\",\"is_private\":false}"
FOLDER=$(id .id)
step "folder create dan (403)" "$DAN" POST /folders "${J[@]}" -d "{\"project_id\":\"$TOWER\",\"name\":\"x\",\"is_private\":false}"
if [ -n "$FOLDER" ]; then
  step "subfolder create" "$BOB" POST /folders "${J[@]}" -d "{\"name\":\"Sub\",\"parent_id\":\"$FOLDER\",\"is_private\":true}"
  SUB=$(id .id)
  step "folder rename" "$BOB" PATCH "/folders/$FOLDER" "${J[@]}" -d '{"name":"Replay renamed"}'
  step "folder visibility" "$BOB" PUT "/folders/$FOLDER/visibility" "${J[@]}" -d '{"is_private":true}'
  step "folder visibility (back)" "$BOB" PUT "/folders/$FOLDER/visibility" "${J[@]}" -d '{"is_private":false}'
  step "folder share candidates" "$BOB" GET "/folders/$FOLDER/share-candidates"
  step "folder share dan viewer" "$BOB" POST "/folders/$FOLDER/grants" "${J[@]}" -d "{\"relation\":\"viewer\",\"subject\":\"user\",\"user_id\":\"$DANID\"}"
  step "folder share dan editor (400 cannot_upload)" "$BOB" POST "/folders/$FOLDER/grants" "${J[@]}" -d "{\"relation\":\"editor\",\"subject\":\"user\",\"user_id\":\"$DANID\"}"
  step "folder grants" "$BOB" GET "/folders/$FOLDER/grants"
  step "document create" "$BOB" POST /documents "${J[@]}" -d "{\"folder_id\":\"$FOLDER\",\"name\":\"Replay doc\",\"mime_type\":\"text/plain\",\"byte_size\":12}"
  DOC=$(id .id)
  step "document create bad size (400)" "$BOB" POST /documents "${J[@]}" -d "{\"folder_id\":\"$FOLDER\",\"name\":\"x\",\"mime_type\":\"text/plain\",\"byte_size\":\"a\"}"
  if [ -n "$DOC" ]; then
    step "documents list" "$BOB" GET "/documents?folder_id=$FOLDER"
    step "documents list dan" "$DAN" GET "/documents?folder_id=$FOLDER"
    step "document get" "$BOB" GET "/documents/$DOC"
    step "document get as eve (404)" "$EVE" GET "/documents/$DOC"
    step "document rename" "$BOB" PATCH "/documents/$DOC" "${J[@]}" -d '{"name":"Replay doc renamed"}'
    step "document share candidates" "$BOB" GET "/documents/$DOC/share-candidates"
    step "document share dan" "$BOB" POST "/documents/$DOC/grants" "${J[@]}" -d "{\"relation\":\"viewer\",\"subject\":\"user\",\"user_id\":\"$DANID\"}"
    step "document grants" "$BOB" GET "/documents/$DOC/grants"
    step "document revoke dan" "$BOB" DELETE "/documents/$DOC/grants" "${J[@]}" -d "{\"relation\":\"viewer\",\"subject\":\"user\",\"user_id\":\"$DANID\"}"
    step "document delete as dan (403)" "$DAN" DELETE "/documents/$DOC"
    step "document delete" "$BOB" DELETE "/documents/$DOC"
  fi
  step "folder revoke dan" "$BOB" DELETE "/folders/$FOLDER/grants" "${J[@]}" -d "{\"relation\":\"viewer\",\"subject\":\"user\",\"user_id\":\"$DANID\"}"
  step "folder delete with child (409/400)" "$BOB" DELETE "/folders/$FOLDER"
  [ -n "$SUB" ] && step "subfolder delete" "$BOB" DELETE "/folders/$SUB"
  step "folder delete" "$BOB" DELETE "/folders/$FOLDER"
fi

# --- albums, photos -------------------------------------------------------------------------------------------
step "albums list bob" "$BOB" GET /albums
step "albums list dan" "$DAN" GET "/albums?project_id=$TOWER"
step "album create" "$BOB" POST /albums "${J[@]}" -d "{\"project_id\":\"$TOWER\",\"name\":\"Replay $RUN\",\"is_private\":false}"
ALBUM=$(id .id)
step "album create dan (403)" "$DAN" POST /albums "${J[@]}" -d "{\"project_id\":\"$TOWER\",\"name\":\"x\",\"is_private\":false}"
if [ -n "$ALBUM" ]; then
  step "album get" "$BOB" GET "/albums/$ALBUM"
  step "album rename" "$BOB" PATCH "/albums/$ALBUM" "${J[@]}" -d '{"name":"Replay renamed"}'
  step "album visibility" "$BOB" PUT "/albums/$ALBUM/visibility" "${J[@]}" -d '{"is_private":true}'
  step "album share candidates" "$BOB" GET "/albums/$ALBUM/share-candidates"
  step "album share dan" "$BOB" POST "/albums/$ALBUM/grants" "${J[@]}" -d "{\"relation\":\"viewer\",\"subject\":\"user\",\"user_id\":\"$DANID\"}"
  step "album grants" "$BOB" GET "/albums/$ALBUM/grants"
  step "photo create" "$BOB" POST /photos "${J[@]}" -d "{\"album_id\":\"$ALBUM\",\"name\":\"Replay photo\",\"mime_type\":\"image/png\",\"byte_size\":99}"
  PHOTO=$(id .id)
  if [ -n "$PHOTO" ]; then
    step "photos list" "$BOB" GET "/photos?album_id=$ALBUM"
    step "photos list dan" "$DAN" GET "/photos?album_id=$ALBUM"
    step "photo get" "$BOB" GET "/photos/$PHOTO"
    step "photo get as eve (404)" "$EVE" GET "/photos/$PHOTO"
    step "photo rename" "$BOB" PATCH "/photos/$PHOTO" "${J[@]}" -d '{"name":"Replay photo renamed"}'
    step "photo share candidates" "$BOB" GET "/photos/$PHOTO/share-candidates"
    step "photo share dan" "$BOB" POST "/photos/$PHOTO/grants" "${J[@]}" -d "{\"relation\":\"viewer\",\"subject\":\"user\",\"user_id\":\"$DANID\"}"
    step "photo grants" "$BOB" GET "/photos/$PHOTO/grants"
    step "photo revoke dan" "$BOB" DELETE "/photos/$PHOTO/grants" "${J[@]}" -d "{\"relation\":\"viewer\",\"subject\":\"user\",\"user_id\":\"$DANID\"}"
    step "photo delete" "$BOB" DELETE "/photos/$PHOTO"
  fi
  step "album revoke dan" "$BOB" DELETE "/albums/$ALBUM/grants" "${J[@]}" -d "{\"relation\":\"viewer\",\"subject\":\"user\",\"user_id\":\"$DANID\"}"
  step "album delete" "$BOB" DELETE "/albums/$ALBUM"
fi
