# GOST Panel (Vue + Go)

首版骨架：
- 后端：Go（REST + WebSocket）
- 前端：Vue3 + Vite（暗色后台风格）

## 已实现
- 仪表盘：在线节点、流量、活跃客户端、告警（WS 实时刷新）
- 节点管理：列表、新增、上线/下线切换
- API 健康检查：`/healthz`

## 目录
- `backend/` Go API 服务
- `frontend/` Vue 前端
- `docker-compose.yml` 一键本地启动

## 运行方式
### 方式1：Docker Compose
```bash
cd gost-panel
docker compose up
```
- 前端: http://localhost:5173
- 后端: http://localhost:8080

### 方式2：分别启动
```bash
# backend
cd backend
go mod tidy
go run main.go

# frontend (new terminal)
cd frontend
npm install
npm run dev
```

## 下一步建议（我可以继续直接补）
1. 登录鉴权（JWT）+ RBAC
2. 客户端管理（套餐、到期、限速）
3. 端口转发与规则管理（ACL）
4. 告警通知（Telegram/Webhook）
5. 接入真实 GOST 配置下发与回滚
