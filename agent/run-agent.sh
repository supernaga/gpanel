#!/bin/sh
set -eu

: "${PANEL_URL:?PANEL_URL is required}"
: "${AGENT_TOKEN:?AGENT_TOKEN is required}"
NODE_UID="${NODE_UID:-$(cat /etc/machine-id 2>/dev/null || hostname)}"
NODE_NAME="${NODE_NAME:-node-$(hostname)}"
NODE_IP="${NODE_IP:-$(hostname -i | awk '{print $1}')}"
AGENT_VERSION="${AGENT_VERSION:-v0.2.0}"

run_cmd() {
  sh -lc "$1" 2>&1
}

install_gost() {
  if command -v gost >/dev/null 2>&1; then
    echo "gost already installed"
    return 0
  fi
  if command -v apt-get >/dev/null 2>&1; then
    run_cmd "apt-get update -y && apt-get install -y wget"
    ARCH=$(uname -m)
    case "$ARCH" in
      x86_64) PKG_ARCH="amd64" ;;
      aarch64) PKG_ARCH="arm64" ;;
      *) PKG_ARCH="amd64" ;;
    esac
    URL="https://github.com/go-gost/gost/releases/latest/download/gost_3.0.0_linux_${PKG_ARCH}.tar.gz"
    run_cmd "cd /tmp && wget -qO gost.tgz $URL && tar -xzf gost.tgz && install -m 0755 gost /usr/local/bin/gost"
  elif command -v apk >/dev/null 2>&1; then
    run_cmd "apk add --no-cache wget tar"
    ARCH=$(uname -m)
    case "$ARCH" in
      x86_64) PKG_ARCH="amd64" ;;
      aarch64) PKG_ARCH="arm64" ;;
      *) PKG_ARCH="amd64" ;;
    esac
    URL="https://github.com/go-gost/gost/releases/latest/download/gost_3.0.0_linux_${PKG_ARCH}.tar.gz"
    run_cmd "cd /tmp && wget -qO gost.tgz $URL && tar -xzf gost.tgz && install -m 0755 gost /usr/local/bin/gost"
  else
    echo "unsupported package manager"
    return 1
  fi
}

ensure_systemd() {
  command -v systemctl >/dev/null 2>&1
}

write_service() {
  NAME="$1"
  EXEC="$2"
  cat >"/etc/systemd/system/gost-${NAME}.service" <<EOF
[Unit]
Description=GOST service ${NAME}
After=network.target

[Service]
Type=simple
ExecStart=${EXEC}
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
EOF
}

apply_forward() {
  PAYLOAD="$1"
  NAME=$(echo "$PAYLOAD" | jq -r '.name // "forward"')
  PROTO=$(echo "$PAYLOAD" | jq -r '.protocol // "tcp"')
  LISTEN=$(echo "$PAYLOAD" | jq -r '.listen // empty')
  TARGET=$(echo "$PAYLOAD" | jq -r '.target // empty')
  if [ -z "$LISTEN" ] || [ -z "$TARGET" ]; then
    echo "missing listen/target"
    return 1
  fi
  install_gost >/dev/null
  ensure_systemd || { echo "systemd required"; return 1; }
  EXEC="/usr/local/bin/gost -L ${PROTO}://${LISTEN}/${TARGET}"
  write_service "$NAME" "$EXEC"
  run_cmd "systemctl daemon-reload && systemctl enable --now gost-${NAME}.service"
}

apply_tunnel() {
  PAYLOAD="$1"
  NAME=$(echo "$PAYLOAD" | jq -r '.name // "tunnel"')
  MODE=$(echo "$PAYLOAD" | jq -r '.mode // "socks5"')
  LISTEN=$(echo "$PAYLOAD" | jq -r '.listen // ":1080"')
  install_gost >/dev/null
  ensure_systemd || { echo "systemd required"; return 1; }
  case "$MODE" in
    socks5) URI="socks5://${LISTEN}" ;;
    http) URI="http://${LISTEN}" ;;
    *) echo "unsupported tunnel mode"; return 1 ;;
  esac
  EXEC="/usr/local/bin/gost -L ${URI}"
  write_service "$NAME" "$EXEC"
  run_cmd "systemctl daemon-reload && systemctl enable --now gost-${NAME}.service"
}

service_action() {
  PAYLOAD="$1"
  ACT="$2"
  NAME=$(echo "$PAYLOAD" | jq -r '.name // ""')
  if [ -z "$NAME" ]; then echo "missing service name"; return 1; fi
  ensure_systemd || { echo "systemd required"; return 1; }
  case "$ACT" in
    start|stop|restart) run_cmd "systemctl ${ACT} gost-${NAME}.service" ;;
    status) run_cmd "systemctl status gost-${NAME}.service --no-pager" ;;
    *) echo "unsupported action"; return 1 ;;
  esac
}

ack() {
  TASK_ID="$1"
  STATUS="$2"
  RESULT_JSON="$3"
  RESULT_ESCAPED=$(printf '%s' "$RESULT_JSON" | jq -Rs .)
  curl -fsS -X POST "$PANEL_URL/api/agent/tasks/$TASK_ID/ack" \
    -H "Authorization: Bearer $AGENT_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"status\":\"$STATUS\",\"result\":$RESULT_ESCAPED}" >/dev/null 2>&1 || true
}

while true; do
  LATENCY_MS=$(( (RANDOM % 90) + 10 ))
  curl -fsS -X POST "$PANEL_URL/api/agent/heartbeat" \
    -H "Authorization: Bearer $AGENT_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"nodeUid\":\"$NODE_UID\",\"nodeName\":\"$NODE_NAME\",\"nodeIp\":\"$NODE_IP\",\"version\":\"$AGENT_VERSION\",\"latencyMs\":$LATENCY_MS,\"region\":\"Unknown\"}" >/dev/null 2>&1 || true

  TASK_JSON=$(curl -fsS "$PANEL_URL/api/agent/tasks/next?nodeUid=$NODE_UID&nodeName=$NODE_NAME" -H "Authorization: Bearer $AGENT_TOKEN" 2>/dev/null || echo '{"task":null}')
  TASK_ID=$(echo "$TASK_JSON" | jq -r '.task.id // empty')
  if [ -n "$TASK_ID" ]; then
    CMD=$(echo "$TASK_JSON" | jq -r '.task.command // ""')
    PAYLOAD=$(echo "$TASK_JSON" | jq -c '.task.payload | fromjson? // {}')
    OUTPUT=""
    STATUS="success"

    case "$CMD" in
      gost.install)
        OUTPUT=$(install_gost 2>&1) || STATUS="failed"
        ;;
      gost.apply_forward)
        OUTPUT=$(apply_forward "$PAYLOAD" 2>&1) || STATUS="failed"
        ;;
      gost.apply_tunnel)
        OUTPUT=$(apply_tunnel "$PAYLOAD" 2>&1) || STATUS="failed"
        ;;
      gost.start)
        OUTPUT=$(service_action "$PAYLOAD" "start" 2>&1) || STATUS="failed"
        ;;
      gost.stop)
        OUTPUT=$(service_action "$PAYLOAD" "stop" 2>&1) || STATUS="failed"
        ;;
      gost.restart)
        OUTPUT=$(service_action "$PAYLOAD" "restart" 2>&1) || STATUS="failed"
        ;;
      gost.status)
        OUTPUT=$(service_action "$PAYLOAD" "status" 2>&1) || STATUS="failed"
        ;;
      *)
        STATUS="failed"
        OUTPUT="unsupported command: $CMD"
        ;;
    esac

    RESULT=$(jq -nc --arg cmd "$CMD" --arg out "$OUTPUT" --arg node "$NODE_NAME" '{ok:true,command:$cmd,node:$node,output:$out}')
    if [ "$STATUS" != "success" ]; then RESULT=$(jq -nc --arg cmd "$CMD" --arg out "$OUTPUT" --arg node "$NODE_NAME" '{ok:false,command:$cmd,node:$node,error:$out}'); fi
    ack "$TASK_ID" "$STATUS" "$RESULT"
  fi

  sleep 10
done
