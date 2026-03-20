# GPanel

GPanel 是一个面向 [GOST](https://github.com/go-gost/gost) 的多节点转发、隧道和链路编排控制面。

当前项目重点不是“做一个大而全的代理面板”，而是先把下面几件事做扎实：

- 节点和 agent 的在线状态管理
- Forward / Tunnel / Chain 的统一建模与下发
- Runtime 巡检和任务运行态观察
- 小规模多机联调和后续结构演进

---

## 当前能力

### 控制面

- PostgreSQL 持久化
- JWT 登录鉴权
- RBAC：`admin` / `viewer`
- 审计日志
- Agent 心跳、任务下发、结果回执
- 任务超时、重试、优先级、单节点派发限制
- 基础告警与静默时段

### 资源模型

- **Nodes**：节点与 agent 在线状态
- **Forwards**：TCP / UDP 端口转发
- **Tunnels**：HTTP / SOCKS5 隧道入口
- **Chains**：多节点链路编排，如 `B -> C -> D`

### Runtime

- 节点状态
- Agent 快照
- Forward / Tunnel / Chain 当前状态
- 最近任务和任务统计

---

## 架构概览

- `backend/`
  Go 后端，负责鉴权、资源管理、任务调度、审计、告警和 WebSocket 指标流
- `agent/`
  Linux host agent，负责心跳、拉取任务、写入 `systemd` 服务并控制本机 GOST 进程
- `frontend/`
  Vue 3 前端，提供 Nodes / Forwards / Tunnels / Chains / Runtime UI
- `deploy/`
  生产部署用脚本和 `docker compose`
- `.github/workflows/deploy.yml`
  推送 `main` 后构建 GHCR 镜像、发布 agent 二进制，并在配置了 secrets 时执行远程部署

---

## 快速开始

### 方式一：服务器一键部署

```bash
curl -fsSL https://raw.githubusercontent.com/supernaga/gpanel/main/deploy/install-panel.sh | bash
```

安装脚本会：

- 克隆或更新仓库到 `/opt/gpanel`
- 生成部署所需密钥并写入 `deploy/.env`
- 优先尝试拉取 GHCR 镜像，失败后回退到本地构建
- 启动 `postgres / backend / frontend`
- 检查前端可达性

默认访问地址：

```text
http://<SERVER_IP>/
```

安装完成后，敏感信息保存在：

```text
/opt/gpanel/deploy/.env
```

其中：

- `ADMIN_USER` 默认是 `admin`
- `ADMIN_PASSWORD` 安装脚本会自动生成
- 节点创建后由 GPanel 一次性签发 node-specific agent token

### 方式二：本地源码启动

根目录下的 `docker-compose.yml` 用于本地开发/联调：

```bash
export POSTGRES_PASSWORD='change-me'
export JWT_SECRET='change-me'
export ADMIN_PASSWORD='change-me'

docker compose up -d --build
```

默认端口：

- 前端：`http://127.0.0.1:5173`
- 后端：`http://127.0.0.1:8080`

---

## Agent 接入

Linux 主机接入：

```bash
curl -fsSL https://raw.githubusercontent.com/supernaga/gpanel/main/deploy/install-agent.sh | bash -s -- \
  --panel http://<PANEL_IP> \
  --token <NODE_AGENT_TOKEN> \
  --name node-01
```

安装脚本会：

- 下载 `gpanel-agent`
- 写入 `/etc/gpanel/agent.env`
- 安装 `gpanel-agent.service`
- 立刻探测 panel `/healthz`

当前脚本直接提供：

- Linux `amd64`
- Linux `arm64`

---

## 核心环境变量

### 部署必需

| 变量 | 说明 |
| --- | --- |
| `POSTGRES_PASSWORD` | PostgreSQL 密码 |
| `JWT_SECRET` | JWT 签名密钥 |
| `ADMIN_PASSWORD` | 初始管理员密码 |

### 常用可选项

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `ADMIN_USER` | `admin` | 初始管理员用户名 |
| `ALLOWED_ORIGINS` | 空 | 允许的浏览器来源，逗号分隔；当前端和后端不在同域时需要设置 |
| `WEBHOOK_URL` | 空 | 告警 webhook |
| `ALERT_OFFLINE_MINUTES` | `2` | 节点离线告警阈值 |
| `ALERT_DEDUPE_MINUTES` | `5` | 告警去重时间窗 |
| `TASK_TIMEOUT_SECONDS` | `300` | 单任务超时 |
| `TASK_MAX_RETRIES` | `3` | 单任务最大重试次数 |
| `TASK_DISPATCH_PER_NODE` | `1` | 单节点并发派发上限 |
| `ALERT_SILENT_HOURS` | 空 | 告警静默时段，例如 `1-7` |
| `IMAGE_PREFIX` | `ghcr.io/supernaga/gpanel` | 镜像前缀 |

---

## 安全基线

- 不要使用弱密码或演示密钥
- 将每个节点的 agent token 视为敏感凭据
- 生产环境建议放在 HTTPS 反向代理后
- 如果前端与后端跨域部署，配置 `ALLOWED_ORIGINS`
- `viewer` 角色是只读角色
- 一键安装脚本会把凭据写入 `deploy/.env`，并收紧到 `600` 权限

---

## 推荐验证路径

### 1. 部署控制面

确认：

- `docker compose ps` 正常
- `backend /healthz` 正常
- Web UI 能登录

### 2. 接入节点

确认：

- `gpanel-agent.service` 正常
- UI 中节点在线
- Runtime 中能看到 agent 快照

### 3. 按顺序验证功能

- Nodes
- Forwards
- Tunnels
- Chains
- Runtime

### 4. 建议优先联调的场景

- 单节点 TCP / UDP 转发
- 单节点 HTTP / SOCKS5 隧道
- 简单链路：`B -> C`
- 多跳链路：`B -> C -> D`

---

## 当前限制

下面这些限制目前依然存在，README 里直接说清楚：

- `desired vs actual` 的完整自动对账还没做完
- 多跳链路更细粒度的端口/目标映射策略还需要继续打磨
- Runtime 中仍有一部分演示性质的数据，尚未全部替换为真实遥测
- 后端核心逻辑仍然偏集中，后续需要继续拆分模块
- 自动化测试和 CI 门禁还不够完整

所以当前版本更适合：

- 持续开发
- 小范围联调
- 提前部署和巡检

---

## 下一步最值得优化的点

如果继续往下做，优先级建议是：

1. 把 Runtime 中的随机演示指标替换成 agent 真实上报数据
2. 拆分 `backend/main.go`，把鉴权、任务队列、资源模型、运行态聚合分成独立模块
3. 补最基本的后端测试和 CI 流程，至少覆盖鉴权、任务状态流转和核心 API
4. 完成 `desired vs actual` 自动对账闭环，而不只是展示当前状态
5. 给 Chain 编排补更稳的目标映射、回滚和错误可观测性

---

## 仓库结构

```text
.
├── agent/                  # Linux host agent
├── backend/                # Go backend
├── deploy/                 # Production deploy scripts and compose files
├── frontend/               # Vue frontend
├── docker-compose.yml      # Local source build / dev compose
└── .github/workflows/      # Build, push and deploy workflow
```

---

## 现状说明

GPanel 现在已经不是最早的 demo 面板，但也还没到“功能完全封板”的阶段。

如果你希望它是一个可继续演进的 GOST 控制面，这个仓库现在的价值主要在于：

- 已经有可运行的控制面骨架
- 已经有可接入的 Linux agent
- 已经有 Forward / Tunnel / Chain 的统一入口
- 已经有 Runtime 巡检和任务状态视图

如果你想直接把它当成熟产品使用，建议先补上上面的“下一步优化”几项。
