# GPanel

GOST **多节点转发 / 隧道 / 链路编排控制面**。

当前方向已经从早期 demo 面板切换为：
- **Nodes**：节点与 agent 在线状态
- **Forwards**：TCP / UDP 端口转发
- **Tunnels**：HTTP / SOCKS5 隧道入口
- **Chains**：多节点链路编排（如 `B -> C -> D`）
- **Runtime**：节点 / 资源 / agent 快照 / 任务运行态巡检

---

## 当前已落地能力

### 控制面
- PostgreSQL 持久化
- JWT 登录鉴权（`/api/auth/login`）
- RBAC（`admin` / `viewer`）
- 审计日志
- Agent 心跳、任务下发、结果回执
- 任务超时 / 重试 / 优先级 / 单节点派发限制
- 基础告警与静默时段

### 资源模型
- **Forwards**：创建 / 编辑 / 删除 / 启停
- **Tunnels**：创建 / 编辑 / 删除 / 启停
- **Chains**：创建 / 编辑 / 删除 / 启停
- 支持 `B -> C -> D` 文本路径的第一版多跳任务拆解

### Runtime
- 节点状态
- Agent 快照（能力 / 服务 inventory）
- 转发状态
- 隧道状态
- 链路状态
- 最近任务与任务统计

> 注意：当前版本已经进入“新结构接管”阶段，但 **desired vs actual 自动对账** 和 **真实多机联调闭环** 仍在继续完善。

---

## 快速安装（Panel）

```bash
curl -fsSL https://raw.githubusercontent.com/supernaga/gpanel/main/deploy/install-panel.sh | bash
```

安装脚本会：
- 拉取或构建镜像
- 启动 `postgres / backend / frontend`
- 检查 `backend /healthz`
- 输出 panel 地址、admin 账号、agent token

---

## 节点接入（Host Agent）

```bash
curl -fsSL https://raw.githubusercontent.com/supernaga/gpanel/main/deploy/install-agent.sh | bash -s -- \
  --panel http://<PANEL_IP> \
  --token <AGENT_TOKEN> \
  --name node-01
```

安装脚本会：
- 下载 `gpanel-agent`
- 安装 systemd 服务
- 尝试探测 panel `/healthz`

---

## 当前推荐测试路径

### 1. 部署控制面（A 机器）
先执行 panel 安装脚本，确认：
- `docker compose ps` 正常
- `backend /healthz` 正常
- 能打开 Web UI

### 2. 接入节点（B / C / D 机器）
安装 host-agent，确认：
- `gpanel-agent.service` 正常
- UI 中节点在线
- Runtime 中能看到 agent 快照

### 3. 在 UI 中按顺序验证
- Nodes
- Forwards
- Tunnels
- Chains
- Runtime

### 4. 建议优先测试的功能
- 单节点 TCP/UDP 转发
- 单节点 HTTP/SOCKS5 隧道
- 简单链路：`B -> C`
- 多跳链路：`B -> C -> D`

---

## 当前限制（实话）
- Runtime 的完整 desired vs actual 自动对账还没做完
- 链路编排更细粒度的端口/目标映射策略还要继续优化
- 真正多机环境下的完整闭环验证还在推进

所以当前版本更适合：
- 持续开发
- 小范围联调
- 提前部署与巡检

---

## 安全建议
- 修改 `POSTGRES_PASSWORD`
- 修改 `JWT_SECRET`
- 修改 `AGENT_TOKEN`
- 修改 `ADMIN_PASSWORD`
- 使用 HTTPS
- 限制面板来源 IP
- 将 agent token 视为敏感凭据
