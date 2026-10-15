#!/usr/bin/env bash
#
# run_integration_demo.sh — self-contained integration test for CloudPing.
#
# Builds the binary (unless SKIP_BUILD=1), generates throwaway mTLS certs if
# missing, spins up a hub + one demo agent + a fake local DNS server, then runs
# a series of API tests against the hub (ICMP/ICMPv6 ping, TCP ping, DNS probe,
# HTTP probe) dispatched through the QUIC tunnel to the agent.
#
# No .env mutation is performed: an ephemeral JWT_SECRET is generated per run,
# and a fresh token is signed via `bin/globalping jwt sign`.
#
# Raw-socket tests (ICMP/TCP ping) require root privileges on non-Linux hosts.
# The script auto-detects privilege (euid==0 or setuid-root binary); if missing,
# those tests are SKIPPED and the exact sudo command to enable them is printed.
#
# Env knobs (all optional):
#   HUB_PUBLIC_PORT=8084   hub public HTTP API
#   HUB_QUIC_PORT=18449    hub JWT-authenticated QUIC listener
#   AGENT_HTTP_PORT=8085   agent plaintext HTTP (debug) listener
#   FAKE_DNS_PORT=15353    fake DNS server (127.0.0.1 only)
#   NODE_NAME=demo-agent-1
#   SKIP_BUILD=1           reuse existing bin/globalping
#   KEEP_RUNNING=1         don't stop hub/agent/dns on exit
#   DISABLE_V6=1           skip the IPv6 ping test
#   VERBOSE=1              stream full NDJSON instead of compact summaries
#
# NOTE: rebuilding replaces bin/globalping, which drops the setuid-root bit.
# After a rebuild, re-apply: sudo chown root:wheel bin/globalping && sudo chmod u+s bin/globalping
# (or use SKIP_BUILD=1 when the code hasn't changed).
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$PROJECT_ROOT"

BIN="$PROJECT_ROOT/bin/globalping"
HUB_PUBLIC_PORT="${HUB_PUBLIC_PORT:-8084}"
HUB_QUIC_PORT="${HUB_QUIC_PORT:-18449}"
AGENT_HTTP_PORT="${AGENT_HTTP_PORT:-8085}"
FAKE_DNS_PORT="${FAKE_DNS_PORT:-15353}"
NODE_NAME="${NODE_NAME:-demo-agent-1}"
HUB_API="http://127.0.0.1:${HUB_PUBLIC_PORT}"

WORKDIR="$(mktemp -d /tmp/cloudping-itest.XXXXXX)"
HUB_LOG="$WORKDIR/hub.log"
AGENT_LOG="$WORKDIR/agent.log"
DNS_LOG="$WORKDIR/dns.log"

N_PASS=0; N_FAIL=0; N_SKIP=0
declare -a FAILED_TESTS=()

c_green=$'\033[32m'; c_red=$'\033[31m'; c_yellow=$'\033[33m'; c_dim=$'\033[2m'; c_reset=$'\033[0m'
log()  { printf '%s[itest]%s %s\n' "$c_dim" "$c_reset" "$*"; }
pass() { N_PASS=$((N_PASS+1)); printf '  %sPASS%s %s\n' "$c_green" "$c_reset" "$1"; }
fail() { N_FAIL=$((N_FAIL+1)); FAILED_TESTS+=("$1"); printf '  %sFAIL%s %s\n' "$c_red" "$c_reset" "$1"; }
skip() { N_SKIP=$((N_SKIP+1)); printf '  %sSKIP%s %s (%s)\n' "$c_yellow" "$c_reset" "$1" "$2"; }

HUB_PID=""; AGENT_PID=""; DNS_PID=""
cleanup() {
  if [ "${KEEP_RUNNING:-0}" = "1" ]; then
    log "KEEP_RUNNING=1, leaving processes up (logs in $WORKDIR)"
    return
  fi
  for pid in "$AGENT_PID" "$HUB_PID" "$DNS_PID"; do
    [ -n "$pid" ] && kill "$pid" 2>/dev/null || true
  done
  wait 2>/dev/null || true
  log "stopped demo processes (logs kept in $WORKDIR)"
}
trap cleanup EXIT

# ---------------------------------------------------------------- preflight
for tool in curl jq python3; do
  command -v "$tool" >/dev/null || { echo "missing dependency: $tool" >&2; exit 2; }
done

if [ "${SKIP_BUILD:-0}" != "1" ]; then
  log "building bin/globalping ..."
  go build -o "$BIN" ./cmd/globalping
fi
[ -x "$BIN" ] || { echo "binary not found: $BIN (run without SKIP_BUILD=1)" >&2; exit 2; }

for f in GeoLite2-City.mmdb GeoLite2-ASN.mmdb; do
  [ -f "$f" ] || { echo "missing $f — run scripts/fetch-mmdb-files.sh first" >&2; exit 2; }
done

# Use the checked-in dev certs from test/certs when present AND unexpired.
# Otherwise generate a throwaway CA + peer pair inside the temp workdir.
# NEVER run confed/hub/gen-ca.sh here: it would overwrite the published hub CA
# that downstream consumers rely on.
CERT_DIR="$PROJECT_ROOT/test/certs"
certs_usable() {
  [ -f "$CERT_DIR/ca.pem" ] && [ -f "$CERT_DIR/peer.pem" ] && [ -f "$CERT_DIR/peer-key.pem" ] \
    && openssl x509 -checkend 0 -noout -in "$CERT_DIR/peer.pem" 2>/dev/null
}
if ! certs_usable; then
  command -v cfssl >/dev/null || { echo "test/certs unusable and cfssl not installed" >&2; exit 2; }
  log "test/certs missing or expired — generating throwaway CA + peer certs in workdir ..."
  CERT_DIR="$WORKDIR/certs"
  mkdir -p "$CERT_DIR"
  cat > "$CERT_DIR/ca.json" <<'EOF'
{"CN":"cloudping-hub","key":{"algo":"ecdsa","size":256},"names":[{"C":"US","L":"San Francisco","O":"Internet Widgets, Inc.","OU":"WWW","ST":"California"}]}
EOF
  cat > "$CERT_DIR/peer.json" <<'EOF'
{"CN":"cloudping-hub","hosts":["cloudping-hub.example.com","localhost","127.0.0.1","::1"],"key":{"algo":"rsa","size":2048},"names":[{"C":"US","L":"San Francisco","O":"Internet Widgets, Inc.","OU":"WWW","ST":"California"}]}
EOF
  ( cd "$CERT_DIR"
    cfssl gencert -initca ca.json | cfssljson -bare ca
    cfssl gencert -ca ca.pem -ca-key ca-key.pem peer.json | cfssljson -bare peer )
fi
CA_PEM="$CERT_DIR/ca.pem"; PEER_PEM="$CERT_DIR/peer.pem"; PEER_KEY_PEM="$CERT_DIR/peer-key.pem"

# raw-socket privilege detection (needed for ICMP/TCP ping on macOS)
RAW_OK=0
if [ "$(id -u)" = "0" ]; then
  RAW_OK=1
elif [ -n "$(find "$BIN" -user root -perm -4000 2>/dev/null)" ]; then
  # setuid-root binary (portable check; avoids stat flavor differences)
  RAW_OK=1
fi

# ephemeral JWT secret + fresh token; exported env beats .env (godotenv won't override)
export JWT_SECRET="$(openssl rand -hex 32)"
TOKEN="$("$BIN" jwt sign 2>/dev/null)"
[ -n "$TOKEN" ] || { echo "failed to sign JWT token" >&2; exit 2; }
export JWT_TOKEN="$TOKEN"
AUTH="Authorization: bearer $TOKEN"

# stop leftovers from previous runs of THIS script (match both ./bin/... and absolute path)
pkill -f 'bin/globalping (hub|agent)' 2>/dev/null || true
sleep 0.5

# ------------------------------------------------------------- fake DNS srv
log "starting fake DNS server on 127.0.0.1:${FAKE_DNS_PORT} ..."
python3 - "$FAKE_DNS_PORT" > "$DNS_LOG" 2>&1 <<'PYEOF' &
import socket, struct, sys
port = int(sys.argv[1])
sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
sock.bind(("127.0.0.1", port))
print(f"fake-dns: listening on 127.0.0.1:{port}", flush=True)
while True:
    data, addr = sock.recvfrom(4096)
    if len(data) < 12:
        continue
    tid = data[0:2]
    flags = bytearray(data[2:4])
    flags[0] |= 0x80          # QR: response
    flags[1] |= 0x80          # RA: recursion available
    qd = struct.unpack(">H", data[4:6])[0]
    offset = 12
    try:
        for _ in range(qd):
            while True:
                l = data[offset]
                if l == 0:
                    offset += 1
                    break
                if l & 0xC0 == 0xC0:
                    offset += 2
                    break
                offset += 1 + l
            offset += 4
    except IndexError:
        continue
    question = data[12:offset]
    header = tid + bytes(flags) + struct.pack(">HHHH", qd, 1, 0, 0)
    answer = b"\xc0\x0c" + struct.pack(">HHIH", 1, 1, 60, 4) + bytes([93, 184, 216, 34])
    sock.sendto(header + question + answer, addr)
PYEOF
DNS_PID=$!

# ------------------------------------------------------------------- launch
log "starting hub (public :${HUB_PUBLIC_PORT}, quic :${HUB_QUIC_PORT}) ..."
nohup "$BIN" hub \
  --server-cert="$PEER_PEM" \
  --server-cert-key="$PEER_KEY_PEM" \
  --peer-ca="$CA_PEM" \
  --public-http-listen-address=":${HUB_PUBLIC_PORT}" \
  --jwt-quic-listen-address=":${HUB_QUIC_PORT}" \
  --min-pkt-interval=30ms \
  --max-pkt-timeout=3000ms \
  --jwt-auth-secret-from-env=JWT_SECRET \
  --maxmind-city-mmdb=GeoLite2-City.mmdb \
  --maxmind-asn-mmdb=GeoLite2-ASN.mmdb \
  > "$HUB_LOG" 2>&1 &
HUB_PID=$!

for i in $(seq 1 30); do
  curl -sf -o /dev/null "$HUB_API/version" && break
  sleep 0.5
  [ "$i" = "30" ] && { echo "hub failed to start; log:" >&2; tail -20 "$HUB_LOG" >&2; exit 2; }
done

log "starting agent '$NODE_NAME' ..."
nohup "$BIN" agent \
  --quic-server-address="127.0.0.1:${HUB_QUIC_PORT}" \
  --server-name=cloudping-hub.example.com \
  --peer-ca="$CA_PEM" \
  --node-name="$NODE_NAME" \
  --exact-location-lat-lon="34.0200392,-118.7413874" \
  --country-code=US --city-name="Los Angeles" --asn=AS12345 --isp="Example LLC" \
  --http-listen-address=":${AGENT_HTTP_PORT}" \
  --maxmind-city-mmdb=GeoLite2-City.mmdb \
  --maxmind-asn-mmdb=GeoLite2-ASN.mmdb \
  --ipregistry-api-endpoint= \
  --ip2location-api-endpoint= \
  --ipinfo-lite-api-endpoint= \
  --ipinfo-provider-search-order=maxmind \
  --dn42-ip2location-api-endpoint= \
  > "$AGENT_LOG" 2>&1 &
AGENT_PID=$!

for i in $(seq 1 40); do
  if curl -sf -H "$AUTH" "$HUB_API/conns" 2>/dev/null | jq -e --arg n "$NODE_NAME" \
      'to_entries[] | select(.value.node_name == $n)' >/dev/null 2>&1; then
    break
  fi
  sleep 0.5
  [ "$i" = "40" ] && { echo "agent failed to register; logs:" >&2; tail -20 "$AGENT_LOG" >&2; exit 2; }
done
log "agent registered with hub"

# ------------------------------------------------------------------ helpers
# stream_ping <name> <query-string> <jq-assertion> [expected-min-events]
stream_ping() {
  local name="$1" qs="$2" assertion="$3" min_events="${4:-1}"
  local out="$WORKDIR/$(echo "$name" | tr ' /' '__').ndjson"
  curl -sN --max-time 45 -H "$AUTH" "$HUB_API/ping?$qs" > "$out" || true
  if [ "${VERBOSE:-0}" = "1" ]; then cat "$out"; fi
  local n
  n="$(wc -l < "$out" | tr -d ' ')"
  if [ "$n" -lt "$min_events" ]; then
    fail "$name — got $n events, expected >= $min_events"
    sed 's/^/    /' "$out" | head -5
    return
  fi
  if jq -se "$assertion" "$out" >/dev/null 2>&1; then
    pass "$name ($n events)"
  else
    fail "$name — assertion failed: $assertion"
    sed 's/^/    /' "$out" | head -5
  fi
}

NO_ERR='all(.[]; (.err == null) and (.error == null))'

# -------------------------------------------------------------------- tests
log "running API tests against $HUB_API ..."

# 1. version endpoint
if curl -sf -H "$AUTH" "$HUB_API/version" | jq -e '.HEAD | length > 0' >/dev/null; then
  pass "GET /version"
else
  fail "GET /version"
fi

# 2. registration attributes visible via /conns
if curl -sf -H "$AUTH" "$HUB_API/conns" | jq -e --arg n "$NODE_NAME" '
    to_entries[] | select(.value.node_name == $n)
    | .value.attributes
    | select(.CapabilityPing == "true" and .SupportQUICTunnel == "true" and .SupportTCP == "true")' >/dev/null; then
  pass "GET /conns shows $NODE_NAME with ping/tcp/quic-tunnel capabilities"
else
  fail "GET /conns capabilities check"
fi

# 3. ICMP ping (raw socket, needs privilege)
if [ "$RAW_OK" = "1" ]; then
  stream_ping "icmp-ping-127.0.0.1" \
    "from=${NODE_NAME}&targets=127.0.0.1&count=3&intervalMs=100&pktTimeoutMs=1500" \
    "length == 3 and all(.[]; .data.Raw[0].LastHop == true) and $NO_ERR" 3

  # 4. large-payload ping: 8000B payload must NOT be truncated by the 65535
  #    initial PMTU guess / MTUCap -> reply size = 8000 + 8 (icmp) + 20 (ipv4)
  stream_ping "icmp-ping-large-payload(pmtu-cap)" \
    "from=${NODE_NAME}&targets=127.0.0.1&count=2&intervalMs=100&pktTimeoutMs=1500&randomPayloadSize=8000" \
    "all(.[]; .data.Raw[0].Size == 8028) and $NO_ERR" 2

  # 5. TCP ping against the hub's own public port (exercises getSrcIP + raw SYN)
  #    Note: on macOS (XNU), raw TCP sockets cannot observe looped-back SYN-ACKs,
  #    so replies never arrive on loopback (ICMP raw sockets don't have this quirk).
  #    -> strict 'received' assertion on Linux; send-side assertion on darwin.
  if [ "$(uname -s)" = "Darwin" ]; then
    stream_ping "tcp-ping-127.0.0.1:${HUB_PUBLIC_PORT}(send-side)" \
      "from=${NODE_NAME}&l4PacketType=tcp&targets=127.0.0.1:${HUB_PUBLIC_PORT}&count=3&intervalMs=100&pktTimeoutMs=1500" \
      "all(.[]; .data.Details.SentAt != null and (.data.Details.SrcIP | tostring | length) > 0 and (.data.Details.SrcPort | type) == \"number\") and $NO_ERR" 3
  else
    stream_ping "tcp-ping-127.0.0.1:${HUB_PUBLIC_PORT}" \
      "from=${NODE_NAME}&l4PacketType=tcp&targets=127.0.0.1:${HUB_PUBLIC_PORT}&count=3&intervalMs=100&pktTimeoutMs=1500" \
      "all(.[]; .data.Type == \"received\" and .data.Details.ReceivedPkt != null) and $NO_ERR" 3
  fi

  # 6. ICMPv6 ping (soft: host may lack v6 loopback)
  if [ "${DISABLE_V6:-0}" != "1" ]; then
    out="$WORKDIR/icmp6.ndjson"
    curl -sN --max-time 45 -H "$AUTH" \
      "$HUB_API/ping?from=${NODE_NAME}&targets=::1&preferV6=true&count=2&intervalMs=100&pktTimeoutMs=1500" > "$out" || true
    if [ "$(wc -l < "$out" | tr -d ' ')" -ge 2 ] && \
       jq -se "all(.[]; .data.Raw[0].LastHop == true) and $NO_ERR" "$out" >/dev/null 2>&1; then
      pass "icmp6-ping-::1"
    else
      skip "icmp6-ping-::1" "no IPv6 loopback or v6 unavailable on this host"
    fi
  fi
else
  for t in "icmp-ping" "icmp-ping-large-payload" "tcp-ping" "icmp6-ping"; do
    skip "$t" "needs root raw sockets"
  done
  echo
  echo "  To enable raw-socket tests, either re-run as root or make the binary setuid-root once:"
  echo "    sudo chown root:wheel $BIN && sudo chmod u+s $BIN"
  echo
fi

# 7. DNS probe against the fake local DNS server (no root needed)
DNS_TARGET='{"corrId":"itest-dns-1","addrport":"127.0.0.1:'"$FAKE_DNS_PORT"'","target":"example.com","queryType":"a","transport":"udp"}'
out="$WORKDIR/dns-probe.ndjson"
curl -sN --max-time 30 -G -H "$AUTH" "$HUB_API/ping" \
  --data-urlencode "from=$NODE_NAME" \
  --data-urlencode "l7PacketType=dns" \
  --data-urlencode "dnsTarget=$DNS_TARGET" > "$out" || true
[ "${VERBOSE:-0}" = "1" ] && cat "$out"
if jq -se "any(.[]; .data.answer_strings != null and (.data.answer_strings | index(\"93.184.216.34\") != null)) and $NO_ERR" "$out" >/dev/null 2>&1; then
  pass "dns-probe udp (fake resolver answered 93.184.216.34)"
else
  fail "dns-probe udp"
  sed 's/^/    /' "$out" | head -5
fi

# 8. HTTP probe: agent fetches its own /version endpoint (no root needed)
HTTP_TARGET='{"url":"http://127.0.0.1:'"$AGENT_HTTP_PORT"'/version"}'
out="$WORKDIR/http-probe.ndjson"
curl -sN --max-time 30 -G -H "$AUTH" "$HUB_API/ping" \
  --data-urlencode "from=$NODE_NAME" \
  --data-urlencode "l7PacketType=http" \
  --data-urlencode "httpTarget=$HTTP_TARGET" > "$out" || true
[ "${VERBOSE:-0}" = "1" ] && cat "$out"
if jq -se "any(.[]; .data.transport != null and .data.transport.Type == \"response\" and .data.transport.Name == \"status\" and (.data.transport.Value | contains(\"200\"))) and $NO_ERR" "$out" >/dev/null 2>&1; then
  pass "http-probe (agent fetched its own /version, status 200)"
else
  fail "http-probe"
  sed 's/^/    /' "$out" | head -5
fi

# ------------------------------------------------------------------ summary
echo
log "done: ${c_green}${N_PASS} passed${c_reset}, ${c_red}${N_FAIL} failed${c_reset}, ${c_yellow}${N_SKIP} skipped${c_reset}"
if [ "$N_FAIL" -gt 0 ]; then
  printf '  failed tests: %s\n' "${FAILED_TESTS[*]}"
  echo "  hub log:    $HUB_LOG"
  echo "  agent log:  $AGENT_LOG"
  exit 1
fi
