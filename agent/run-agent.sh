#!/bin/sh
set -eu

: "${PANEL_URL:?PANEL_URL is required}"
: "${AGENT_TOKEN:?AGENT_TOKEN is required}"
NODE_UID="${NODE_UID:-$(cat /etc/machine-id 2>/dev/null || hostname)}"
NODE_NAME="${NODE_NAME:-node-$(hostname)}"
NODE_IP="${NODE_IP:-$(hostname -i | awk '{print $1}')}"
AGENT_VERSION="${AGENT_VERSION:-v0.1.0}"

while true; do
  LATENCY_MS=$(( (RANDOM % 90) + 10 ))
  curl -fsS -X POST "$PANEL_URL/api/agent/heartbeat" \
    -H "Authorization: Bearer $AGENT_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"nodeUid\":\"$NODE_UID\",\"nodeName\":\"$NODE_NAME\",\"nodeIp\":\"$NODE_IP\",\"version\":\"$AGENT_VERSION\",\"latencyMs\":$LATENCY_MS,\"region\":\"Unknown\"}" || true

  TASK_JSON=$(curl -fsS "$PANEL_URL/api/agent/tasks/next?nodeUid=$NODE_UID&nodeName=$NODE_NAME" -H "Authorization: Bearer $AGENT_TOKEN" || echo '{"task":null}')
  TASK_ID=$(echo "$TASK_JSON" | jq -r '.task.id // empty')
  if [ -n "$TASK_ID" ]; then
    RESULT=$(jq -nc --arg node "$NODE_NAME" --arg at "$(date -Iseconds)" '{ok:true,node:$node,at:$at}')
    curl -fsS -X POST "$PANEL_URL/api/agent/tasks/$TASK_ID/ack" \
      -H "Authorization: Bearer $AGENT_TOKEN" \
      -H "Content-Type: application/json" \
      -d "{\"status\":\"success\",\"result\":$(printf '%s' "$RESULT" | jq -Rs .)}" || true
  fi

  sleep 15
done
