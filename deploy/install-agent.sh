#!/usr/bin/env bash
set -e
PANEL=""
TOKEN=""
NAME="node-$(hostname)"
BINARY_URL="${BINARY_URL:-https://github.com/supernaga/gpanel/releases/latest/download/gpanel-agent-linux-amd64}"

while [[ $# -gt 0 ]]; do
case "$1" in
--panel) PANEL="$2"; shift 2;;
--token) TOKEN="$2"; shift 2;;
--name) NAME="$2"; shift 2;;
--binary-url) BINARY_URL="$2"; shift 2;;
*) shift;;
esac
done

if [ -z "$PANEL" ] || [ -z "$TOKEN" ]; then
  echo "Usage: bash install-agent.sh --panel http://PANEL_IP --token NODE_AGENT_TOKEN [--name node-01]"
  exit 1
fi

command -v curl >/dev/null 2>&1 || (apt-get update && apt-get install -y curl)
mkdir -p /etc/gpanel
cat >/etc/gpanel/agent.env <<EOF
PANEL_URL=$PANEL
AGENT_TOKEN=$TOKEN
NODE_UID=$(cat /etc/machine-id 2>/dev/null || hostname)
NODE_NAME=$NAME
NODE_IP=$(hostname -I | awk '{print $1}')
AGENT_VERSION=v0.3.0
EOF

ARCH=$(uname -m)
case "$ARCH" in
  x86_64) URL="$BINARY_URL" ;;
  aarch64) URL="${BINARY_URL/linux-amd64/linux-arm64}" ;;
  *) URL="$BINARY_URL" ;;
esac
curl -fsSL "$URL" -o /usr/local/bin/gpanel-agent
chmod +x /usr/local/bin/gpanel-agent

cat >/etc/systemd/system/gpanel-agent.service <<'EOF'
[Unit]
Description=GPanel Host Agent
After=network.target

[Service]
EnvironmentFile=/etc/gpanel/agent.env
ExecStart=/usr/local/bin/gpanel-agent
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now gpanel-agent.service
systemctl status gpanel-agent.service --no-pager | sed -n '1,12p'
echo "[INFO] Probing panel healthz..."
curl -fsS "$PANEL/healthz" >/dev/null && echo "[OK] panel reachable" || echo "[WARN] panel healthz probe failed"
echo "[OK] host agent installed"
echo "[INFO] After node online, verify in UI: Nodes -> Forwards/Tunnels -> Chains -> Runtime"
