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
