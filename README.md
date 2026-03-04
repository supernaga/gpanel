# GPanel

一个用于管理多节点的 Panel + Agent 项目（当前为 **MVP / 演示版后端**）。

## 一键安装 Panel（推荐）

```bash
curl -fsSL https://raw.githubusercontent.com/supernaga/gpanel/main/deploy/install-panel.sh | bash
```

安装完成后访问：

- `http://服务器IP`

> 默认会自动生成随机 `POSTGRES_PASSWORD` / `JWT_SECRET` / `AGENT_TOKEN`，并写入 `deploy/.env`。

## 节点接入（安装 Agent）

```bash
curl -fsSL https://raw.githubusercontent.com/supernaga/gpanel/main/deploy/install-agent.sh | bash -s -- \
  --panel http://<PANEL_IP>:8080 \
  --token <AGENT_TOKEN> \
  --name node-01
```

## 手动部署（Docker Compose）

```bash
cd deploy
cp .env.example .env   # 若没有可直接创建，参考下方环境变量说明
docker compose up -d --build
```

### 关键环境变量

- `POSTGRES_PASSWORD`：PostgreSQL 密码（必须修改）
- `JWT_SECRET`：JWT 签名密钥（必须修改）
- `AGENT_TOKEN`：Agent 注册令牌（必须修改）
- `IMAGE_PREFIX`：镜像前缀（默认 `ghcr.io/supernaga/gpanel`）
- `CORS_ALLOW_ORIGIN`：后端 CORS 白名单（默认 `*`，生产建议改成前端域名）
- `WS_ALLOW_ORIGIN`：WebSocket Origin 白名单（默认 `*`，生产建议改成前端域名）

## 当前状态说明（重要）

当前 `backend/main.go` 仍是演示型实现：

- 节点数据为内存模拟，不持久化到 PostgreSQL
- 未启用 JWT 鉴权流程

`deploy` 目录已预留数据库与密钥配置，后续会逐步接入生产能力。

## 安全建议（强烈）

首次安装后请立即修改：

- `POSTGRES_PASSWORD`
- `JWT_SECRET`
- `AGENT_TOKEN`

并避免长期使用默认/弱口令。
