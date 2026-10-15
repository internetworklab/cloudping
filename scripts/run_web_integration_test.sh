#!/usr/bin/env bash
#
# run_web_integration_test.sh — integration test for `globalping web`.
#
# Verifies that the web assets embedded into the binary via go:embed are served
# correctly (mirroring the old nginx `try_files $uri $uri.html $uri/ =404;`
# semantics), that the JWT auth proxy behavior is intact end-to-end (redirect
# -> visitor login -> authenticated access), and that reverse-proxy mode
# (--default-backend) still works.
#
# Three server instances are launched on loopback:
#   1. static mode  (WEB_PORT)        embedded assets, auth wrapper disabled
#   2. auth mode    (WEB_AUTH_PORT)   embedded assets + JWT auth + whitelist
#   3. proxy mode   (PROXY_PORT)      reverse proxy to the built-in testing backend
#
# No .env mutation is performed: an ephemeral JWT_SECRET is generated per run.
#
# Env knobs (all optional):
#   WEB_PORT=18092                  static mode listen port
#   WEB_AUTH_PORT=18093             auth mode listen port
#   PROXY_PORT=18095                proxy mode listen port
#   TESTING_BACKEND_ADDR=127.0.0.1:18094
#   SKIP_BUILD=1                    reuse existing bin/globalping
#   SKIP_WEB_BUILD=1                never run npm; fail if web/out is missing
#   FORCE_WEB_BUILD=1               rebuild web/out even if it exists
#   KEEP_RUNNING=1                  don't stop the servers on exit
#   VERBOSE=1                       print server logs at the end
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

BIN="$PROJECT_ROOT/bin/globalping"
WEB_PORT="${WEB_PORT:-18092}"
WEB_AUTH_PORT="${WEB_AUTH_PORT:-18093}"
PROXY_PORT="${PROXY_PORT:-18095}"
TESTING_BACKEND_ADDR="${TESTING_BACKEND_ADDR:-127.0.0.1:18094}"

STATIC_URL="http://127.0.0.1:${WEB_PORT}"
AUTH_URL="http://127.0.0.1:${WEB_AUTH_PORT}"
PROXY_URL="http://127.0.0.1:${PROXY_PORT}"

WORKDIR="$(mktemp -d /tmp/cloudping-web-itest.XXXXXX)"
STATIC_LOG="$WORKDIR/web-static.log"
AUTH_LOG="$WORKDIR/web-auth.log"
PROXY_LOG="$WORKDIR/web-proxy.log"

N_PASS=0; N_FAIL=0; N_SKIP=0
declare -a FAILED_TESTS=()

c_green=$'\033[32m'; c_red=$'\033[31m'; c_yellow=$'\033[33m'; c_dim=$'\033[2m'; c_reset=$'\033[0m'
log()  { printf '%s[itest]%s %s\n' "$c_dim" "$c_reset" "$*"; }
pass() { N_PASS=$((N_PASS+1)); printf '  %sPASS%s %s\n' "$c_green" "$c_reset" "$1"; }
fail() { N_FAIL=$((N_FAIL+1)); FAILED_TESTS+=("$1"); printf '  %sFAIL%s %s\n' "$c_red" "$c_reset" "$1"; }
skip() { N_SKIP=$((N_SKIP+1)); printf '  %sSKIP%s %s (%s)\n' "$c_yellow" "$c_reset" "$1" "$2"; }

STATIC_PID=""; AUTH_PID=""; PROXY_PID=""
cleanup() {
  if [ "${KEEP_RUNNING:-0}" = "1" ]; then
    log "KEEP_RUNNING=1, leaving processes up (logs in $WORKDIR)"
    return
  fi
  for pid in "$STATIC_PID" "$AUTH_PID" "$PROXY_PID"; do
    [ -n "$pid" ] && kill "$pid" 2>/dev/null || true
  done
  wait 2>/dev/null || true
  log "stopped demo processes (logs kept in $WORKDIR)"
}
trap cleanup EXIT

# ---------------------------------------------------------------- preflight
for tool in curl jq; do
  command -v "$tool" >/dev/null || { echo "missing dependency: $tool" >&2; exit 2; }
done

# Frontend static export: rebuild when forced or missing.
if [ "${FORCE_WEB_BUILD:-0}" = "1" ] || [ ! -f web/out/index.html ]; then
  if [ "${SKIP_WEB_BUILD:-0}" = "1" ]; then
    echo "web/out/index.html missing and SKIP_WEB_BUILD=1" >&2
    exit 2
  fi
  command -v npm >/dev/null || {
    echo "web/out/index.html missing and npm not installed; build the frontend first (cd web && npm ci && npm run build)" >&2
    exit 2
  }
  log "building frontend assets (npm ci && npm run build) ..."
  ( cd web && npm ci && npm run build )
  # next build wipes out/; re-create the tracked placeholder so git stays clean
  touch web/out/.gitkeep
fi
[ -f web/out/index.html ] || { echo "web/out/index.html not found" >&2; exit 2; }

if [ "${SKIP_BUILD:-0}" != "1" ]; then
  log "building bin/globalping (with embedded web assets) ..."
  go build -o "$BIN" ./cmd/globalping
fi
[ -x "$BIN" ] || { echo "binary not found: $BIN (run without SKIP_BUILD=1)" >&2; exit 2; }

# ephemeral JWT secret; exported env beats .env (godotenv won't override)
export JWT_SECRET="$(openssl rand -hex 32)"

# stop leftovers from previous runs of THIS script
pkill -f 'bin/globalping web' 2>/dev/null || true
sleep 0.5

# dynamic fixtures from the built export (names are content-hashed)
NEXT_CHUNK="$(cd web/out && find _next -name '*.js' | head -1 || true)"
FLAG_FILE="$(cd web/out && find flags -name '*.svg' 2>/dev/null | head -1 || true)"
[ -n "$NEXT_CHUNK" ] || { echo "no _next js chunks found in web/out" >&2; exit 2; }

# ------------------------------------------------------------------- launch
log "starting static-mode server on :${WEB_PORT} (embedded assets, auth disabled) ..."
nohup "$BIN" web \
  --jwt-auth-secret-from-env=JWT_SECRET \
  --listen-address=":${WEB_PORT}" \
  --redirect-if-no-auth= \
  > "$STATIC_LOG" 2>&1 &
STATIC_PID=$!

log "starting auth-mode server on :${WEB_AUTH_PORT} (embedded assets + JWT auth) ..."
nohup "$BIN" web \
  --jwt-auth-secret-from-env=JWT_SECRET \
  --listen-address=":${WEB_AUTH_PORT}" \
  --add-white-list-path=/_next/ \
  --add-white-list-path=/login/as/ \
  --add-white-list-path=/login/ \
  --add-white-list-path=/login \
  > "$AUTH_LOG" 2>&1 &
AUTH_PID=$!

log "starting proxy-mode server on :${PROXY_PORT} (reverse proxy to testing backend $TESTING_BACKEND_ADDR) ..."
nohup "$BIN" web \
  --jwt-auth-secret-from-env=JWT_SECRET \
  --listen-address=":${PROXY_PORT}" \
  --redirect-if-no-auth= \
  --default-backend="http://${TESTING_BACKEND_ADDR}" \
  --launch-testing-backend="${TESTING_BACKEND_ADDR}" \
  > "$PROXY_LOG" 2>&1 &
PROXY_PID=$!

wait_for() { # <url> <want-status> <logfile> <pid>
  local url="$1" want="$2" logf="$3" pid="$4"
  for i in $(seq 1 40); do
    [ "$(curl -s -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || true)" = "$want" ] && return 0
    kill -0 "$pid" 2>/dev/null || break
    sleep 0.5
  done
  echo "server failed to start (waiting for $want at $url); log:" >&2
  tail -20 "$logf" >&2
  exit 2
}

wait_for "$STATIC_URL/" 200 "$STATIC_LOG" "$STATIC_PID"
wait_for "$AUTH_URL/login" 200 "$AUTH_LOG" "$AUTH_PID"
wait_for "$PROXY_URL/" 200 "$PROXY_LOG" "$PROXY_PID"

# ------------------------------------------------------------------ helpers
check_status() { # <name> <url> <want-status> [curl args...]
  local name="$1" url="$2" want="$3"; shift 3
  local got
  got="$(curl -s -o /dev/null -w '%{http_code}' "$url" "$@")"
  if [ "$got" = "$want" ]; then
    pass "$name ($want)"
  else
    fail "$name — got status $got, want $want"
  fi
}

check_body() { # <name> <url> <needle> [curl args...]
  local name="$1" url="$2" needle="$3"; shift 3
  local body
  body="$(curl -s "$url" "$@")"
  if printf '%s' "$body" | grep -q -- "$needle"; then
    pass "$name"
  else
    fail "$name — body missing: $needle"
    printf '%s\n' "$body" | head -5 | sed 's/^/    /'
  fi
}

check_headers() { # <name> <url> <want-status> <header-needle> [curl args...]
  local name="$1" url="$2" want="$3" needle="$4"; shift 4
  local resp got
  resp="$(curl -s -D - -o /dev/null "$url" "$@")"
  got="$(printf '%s' "$resp" | head -1 | awk '{print $2}')"
  if [ "$got" != "$want" ]; then
    fail "$name — got status $got, want $want"
    return
  fi
  if printf '%s' "$resp" | grep -qi -- "$needle"; then
    pass "$name"
  else
    fail "$name — header missing: $needle"
    printf '%s\n' "$resp" | head -10 | sed 's/^/    /'
  fi
}

# -------------------------------------------------------------------- tests
log "running tests ..."

# --- 1. static mode: embedded asset serving (try_files semantics)
check_status  "static: GET /"                        "$STATIC_URL/" 200
check_headers "static: / is text/html"               "$STATIC_URL/" 200 '^content-type: text/html'
check_status  "static: GET /login (clean URL)"       "$STATIC_URL/login" 200
check_status  "static: GET /demos/demo4 (clean URL)" "$STATIC_URL/demos/demo4" 200
check_status  "static: GET /login.txt (RSC payload)" "$STATIC_URL/login.txt" 200
check_status  "static: GET /_next chunk"             "$STATIC_URL/$NEXT_CHUNK" 200
check_headers "static: _next assets are immutable"   "$STATIC_URL/$NEXT_CHUNK" 200 '^cache-control: .*immutable'
check_status  "static: GET /favicon.ico"             "$STATIC_URL/favicon.ico" 200
if [ -n "$FLAG_FILE" ]; then
  check_status "static: GET /$FLAG_FILE"             "$STATIC_URL/$FLAG_FILE" 200
else
  skip "static: flags svg" "no flags/*.svg in web/out"
fi
check_status  "static: GET /version.txt"             "$STATIC_URL/version.txt" 200
check_body    "static: /version.txt has HEAD"        "$STATIC_URL/version.txt" 'HEAD:'
check_headers "static: GET /missing -> 404 page"     "$STATIC_URL/no/such/page" 404 '^content-type: text/html'
check_status  "static: POST / -> 405"                "$STATIC_URL/" 405 -X POST
check_status  "static: GET /.gitkeep -> 404"         "$STATIC_URL/.gitkeep" 404
check_status  "static: GET /.version.txt -> 404"     "$STATIC_URL/.version.txt" 404
check_status  "static: range request -> 206"         "$STATIC_URL/login" 206 -H 'Range: bytes=0-10'

# --- 2. auth mode: JWT proxy behavior on top of embedded assets
check_headers "auth: GET / without creds -> redirect" "$AUTH_URL/" 307 '^location: .*/login'
check_status  "auth: GET /login is whitelisted"       "$AUTH_URL/login" 200
check_status  "auth: GET /_next chunk is whitelisted" "$AUTH_URL/$NEXT_CHUNK" 200

# visitor login flow: /login/as/visitor issues a jwt cookie
VISIT_RESP="$(curl -s -D - -o /dev/null "$AUTH_URL/login/as/visitor")"
VISIT_STATUS="$(printf '%s' "$VISIT_RESP" | head -1 | awk '{print $2}')"
VISIT_TOKEN="$(printf '%s' "$VISIT_RESP" | grep -i '^set-cookie:' | head -1 \
  | sed -E 's/^[Ss]et-[Cc]ookie:[[:space:]]*jwt=([^;]+).*/\1/' | tr -d '\r')"
if [ "$VISIT_STATUS" = "307" ] && [ -n "$VISIT_TOKEN" ]; then
  pass "auth: GET /login/as/visitor issues token (307 + jwt cookie)"
else
  fail "auth: visitor login — status=$VISIT_STATUS, token empty=${VISIT_TOKEN:+no}"
fi

if [ -n "$VISIT_TOKEN" ]; then
  check_status "auth: GET / with bearer token"        "$AUTH_URL/" 200 -H "Authorization: Bearer $VISIT_TOKEN"
  check_status "auth: GET / with jwt cookie"          "$AUTH_URL/" 200 -H "Cookie: jwt=$VISIT_TOKEN"
  check_body   "auth: GET /login/profile"             "$AUTH_URL/login/profile" 'visitor-' -H "Authorization: Bearer $VISIT_TOKEN"
fi
check_status  "auth: GET / with garbage token -> redirect" "$AUTH_URL/" 307 -H "Authorization: Bearer not-a-token"

# --- 3. proxy mode: reverse proxy to the testing backend
check_body    "proxy: GET / -> testing backend"       "$PROXY_URL/" 'Hello, World!'

# ------------------------------------------------------------------ summary
echo
log "done: ${c_green}${N_PASS} passed${c_reset}, ${c_red}${N_FAIL} failed${c_reset}, ${c_yellow}${N_SKIP} skipped${c_reset}"
if [ "${VERBOSE:-0}" = "1" ]; then
  for f in "$STATIC_LOG" "$AUTH_LOG" "$PROXY_LOG"; do
    echo "--- $f ---"; sed 's/^/  /' "$f" | tail -30
  done
fi
if [ "$N_FAIL" -gt 0 ]; then
  printf '  failed tests: %s\n' "${FAILED_TESTS[*]}"
  echo "  static log: $STATIC_LOG"
  echo "  auth log:   $AUTH_LOG"
  echo "  proxy log:  $PROXY_LOG"
  exit 1
fi
