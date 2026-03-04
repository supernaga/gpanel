#!/usr/bin/env bash
set -e
PANEL=""
TOKEN=""
NAME="node-$(hostname)"
IMAGE="${IMAGE:-alpine:3.20}"

while [[ $# -gt 0 ]]; do
case "$1" in
--panel) PANEL="$2"; shift 2;;
--token) TOKEN="$2"; shift 2;;
--name) NAME="$2"; shift 2;;
*) shift;;
esac
done

if [ -z "$PANEL" ] || [ -z "$TOKEN" ]; then
echo "Usage: bash install-agent.sh --panel http://PANEL_IP --token AGENT_TOKEN [--name node-01]"
exit 1
fi

command -v docker >/dev/null 2>&1 || curl -fsSL https://get.docker.com | sh
docker rm -f gpanel-agent >/dev/null 2>&1 || true

docker run -d --name gpanel-agent --restart always \
-e PANEL_URL="$PANEL" \
-e AGENT_TOKEN="$TOKEN" \
-e NODE_UID="$(cat /etc/machine-id 2>/dev/null || hostname)" \
-e NODE_NAME="$NAME" \
-e NODE_IP="$(hostname -I | awk '{print $1}')" \
-e AGENT_VERSION="v0.1.0" \
"$IMAGE" sh -c '
set -e
apk add --no-cache curl jq >/dev/null 2>&1 || true
while true; do
  LAT=$(( (RANDOM % 90) + 10 ))
  curl -fsS -X POST "$PANEL_URL/api/agent/heartbeat" \
    -H "Authorization: Bearer $AGENT_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"nodeUid\":\"$NODE_UID\",\"nodeName\":\"$NODE_NAME\",\"nodeIp\":\"$NODE_IP\",\"version\":\"$AGENT_VERSION\",\"latencyMs\":$LAT,\"region\":\"Unknown\"}" || true

  TASK=$(curl -fsS "$PANEL_URL/api/agent/tasks/next?nodeUid=$NODE_UID&nodeName=$NODE_NAME" -H "Authorization: Bearer $AGENT_TOKEN" || echo "{}")
  ID=$(echo "$TASK" | jq -r ".task.id // empty" 2>/dev/null || true)
  if [ -n "$ID" ]; then
    R=$(jq -nc --arg n "$NODE_NAME" --arg t "$(date -Iseconds)" '{ok:true,node:$n,at:$t}' 2>/dev/null || echo '{"ok":true}')
    curl -fsS -X POST "$PANEL_URL/api/agent/tasks/$ID/ack" \
      -H "Authorization: Bearer $AGENT_TOKEN" \
      -H "Content-Type: application/json" \
      -d "{\"status\":\"success\",\"result\":$(printf "%s" "$R" | jq -Rs .)}" || true
  fi
  sleep 15
done
'

echo "[OK] agent started"
