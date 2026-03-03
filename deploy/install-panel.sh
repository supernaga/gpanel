#!/usr/bin/env bash
set -e
REPO_URL="${REPO_URL:-https://github.com/supernaga/gpanel.git}"
APP_DIR="${APP_DIR:-/opt/gpanel}"
BRANCH="${BRANCH:-main}"

rand() { tr -dc A-Za-z0-9 </dev/urandom | head -c 32; }

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

POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-$(rand)}"
JWT_SECRET="${JWT_SECRET:-$(rand)}"
AGENT_TOKEN="${AGENT_TOKEN:-$(rand)}"
IMAGE_PREFIX="${IMAGE_PREFIX:-ghcr.io/supernaga/gpanel}"

cat > deploy/.env <<EOT
POSTGRES_PASSWORD=$POSTGRES_PASSWORD
JWT_SECRET=$JWT_SECRET
AGENT_TOKEN=$AGENT_TOKEN
IMAGE_PREFIX=$IMAGE_PREFIX
EOT

cd deploy
docker compose up -d --build

IP="$(hostname -I | awk "{print \$1}")"
echo ""
echo "[OK] Panel started: http://$IP"
echo "[INFO] Secrets saved in: $APP_DIR/deploy/.env"
echo "[INFO] Agent token: $AGENT_TOKEN"
