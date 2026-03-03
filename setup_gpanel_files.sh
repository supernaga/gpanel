#!/usr/bin/env bash
set -e
mkdir -p deploy .github/workflows

cat > deploy/install-panel.sh <<'EOF'
#!/usr/bin/env bash
set -e
REPO_URL="${REPO_URL:-https://github.com/supernaga/gpanel.git}"
APP_DIR="${APP_DIR:-/opt/gpanel}"
BRANCH="${BRANCH:-main}"

command -v git >/dev/null 2>&1 || (apt-get update && apt-get install -y git curl)
command -v docker >/dev/null 2>&1 || curl -fsSL https://get.docker.com | sh

mkdir -p "$APP_DIR"
if [ ! -d "$APP_DIR/.git" ]; then
git clone -b "$BRANCH" "$REPO_URL" "$APP_DIR"
else
cd "$APP_DIR"
git fetch origin
git reset --hard "origin/$BRANCH"
fi

cd "$APP_DIR"
cat > deploy/.env <<EOT
POSTGRES_PASSWORD=\${POSTGRES_PASSWORD:-gpanel123}
JWT_SECRET=\${JWT_SECRET:-change_me_jwt}
AGENT_TOKEN=\${AGENT_TOKEN:-change_me_agent}
IMAGE_PREFIX=\${IMAGE_PREFIX:-ghcr.io/supernaga/gpanel}
EOT

cd deploy
docker compose up -d --build
echo "[OK] panel started"
EOF

cat > deploy/install-agent.sh <<'EOF'
#!/usr/bin/env bash
set -e
PANEL=""
TOKEN=""
NAME="node-$(hostname)"
IMAGE="${IMAGE:-ghcr.io/supernaga/gpanel-agent:latest}"

while [[ $# -gt 0 ]]; do
case "$1" in
--panel) PANEL="$2"; shift 2;;
--token) TOKEN="$2"; shift 2;;
--name) NAME="$2"; shift 2;;
*) shift;;
esac
done

if [ -z "$PANEL" ] || [ -z "$TOKEN" ]; then
echo "Usage: bash install-agent.sh --panel http://PANEL_IP:8080 --token AGENT_TOKEN [--name node-01]"
exit 1
fi

command -v docker >/dev/null 2>&1 || curl -fsSL https://get.docker.com | sh
docker rm -f gpanel-agent >/dev/null 2>&1 || true
docker run -d --name gpanel-agent --restart always \
-e PANEL_URL="$PANEL" \
-e AGENT_TOKEN="$TOKEN" \
-e NODE_UID="$(cat /etc/machine-id 2>/dev/null || hostname)" \
-e NODE_NAME="$NAME" \
-e NODE_IP="$(hostname -I | awk "{print \$1}")" \
-e AGENT_VERSION="v0.1.0" \
"$IMAGE"

echo "[OK] agent started"
EOF

cat > deploy/docker-compose.yml <<'EOF'
version: "3.9"
services:
postgres:
image: postgres:16
environment:
POSTGRES_DB: gpanel
POSTGRES_USER: gpanel
POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
volumes:
- pgdata:/var/lib/postgresql/data
restart: always

backend:
image: ${IMAGE_PREFIX}-server:latest
environment:
PORT: 8080
DB_DSN: postgres://gpanel:${POSTGRES_PASSWORD}@postgres:5432/gpanel?sslmode=disable
JWT_SECRET: ${JWT_SECRET}
AGENT_TOKEN: ${AGENT_TOKEN}
depends_on:
- postgres
restart: always

frontend:
image: ${IMAGE_PREFIX}-web:latest
ports:
- "80:80"
depends_on:
- backend
restart: always

volumes:
pgdata:
EOF

cat > .github/workflows/deploy.yml <<'EOF'
name: Deploy GPanel
on:
push:
branches: [ "main" ]
jobs:
deploy:
runs-on: ubuntu-latest
steps:
- run: echo "workflow placeholder"
EOF

chmod +x deploy/install-panel.sh deploy/install-agent.sh
echo "OK"

