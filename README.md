# GPanel

GOST 多节点管理面板（Panel）+ Agent 接入。

## 第二版（当前）能力

- PostgreSQL 持久化（节点、客户端、转发、规则、告警、用户）
- JWT 登录鉴权（`/api/auth/login`）
- 角色权限（`admin` 可写，`viewer` 只读）
- Agent 心跳接口（`/api/agent/heartbeat`，Bearer `AGENT_TOKEN`）
- 基础告警引擎（节点离线 >2 分钟自动告警）+ 5 分钟去重抑制
- 可选 Webhook 告警转发（`WEBHOOK_URL`）
- Agent 任务通道（管理端下发任务，Agent 拉取并回执）
- 任务超时/重试（可配置 timeout + max retries）
- 任务优先级队列 + 单节点派发并发限制
- 结构化任务负载/结果（要求 JSON 字符串）
- 审计日志（关键写操作记录，支持按用户/动作/时间筛选）
- RBAC 用户管理（admin/viewer，支持新增与角色切换）
- 告警策略可配置（离线阈值、去重窗口、任务超时与重试、静默时段）
- 前端模块：仪表盘 / 节点 / 客户端 / 端口转发 / 规则 / 告警 / Agent任务 / 用户与审计

---

## 快速安装（推荐）

```bash
curl -fsSL https://raw.githubusercontent.com/supernaga/gpanel/main/deploy/install-panel.sh | bash
```

安装完成后访问：

- `http://服务器IP`

安装脚本会输出：

- Agent Token
- Admin 用户名/密码

> 重要：请妥善保存 `deploy/.env`。

---

## 节点接入

```bash
curl -fsSL https://raw.githubusercontent.com/supernaga/gpanel/main/deploy/install-agent.sh | bash -s -- \
  --panel http://<PANEL_IP>:8080 \
  --token <AGENT_TOKEN> \
  --name node-01
```

---

## Docker 手动安装

```bash
cd deploy
docker compose up -d --build
```

---

## 安全建议（强烈）

首次安装后务必修改：

- `POSTGRES_PASSWORD`
- `JWT_SECRET`
- `AGENT_TOKEN`
- `ADMIN_PASSWORD`

并建议：

- 用反向代理加 HTTPS
- 限制 `8080/80` 来源 IP
- 配置 `WEBHOOK_URL` 仅发往可信端点
