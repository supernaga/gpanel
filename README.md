# GPanel

GOST **多节点转发 / 隧道 / 链路编排控制面**。

当前方向已经从早期 demo 面板切换为：
- **Nodes**：节点与 agent 在线状态
- **Forwards**：TCP / UDP 端口转发
- **Tunnels**：HTTP / SOCKS5 隧道入口
- **Chains**：多节点链路编排（如 `B -> C -> D`）
- **Runtime**：节点 / 资源 / 任务运行态巡检

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
- **Forwards**
  - 创建 / 编辑 / 删除 / 启停
- **Tunnels**
  - 创建 / 编辑 / 删除 / 启停
- **Chains**
  - 创建 / 编辑 / 删除 / 启停
  - 支持 `B -> C -> D` 文本路径的第一版多跳任务拆解

### Runtime
- 节点状态
- 转发状态
- 隧道状态
- 链路状态
- 最近任务与任务统计

> 注意：当前版本已经进入“新结构接管”阶段，但 **desired vs actual 对账** 和 **真实多机联调闭环** 仍在继续完善。

---

## 快速安装（Panel）

```bash
curl -fsSL https://raw.githubusercontent.com/supernaga/gpanel/main/deploy/install-panel.sh | bash
```

安装完成后访问：
- `http://服务器IP`

安装脚本会输出：
- Admin 用户名 / 密码
- Agent Token

---

## 节点接入（Host Agent）

```bash
curl -fsSL https://raw.githubusercontent.com/supernaga/gpanel/main/deploy/install-agent.sh | bash -s -- \
  --panel http://<PANEL_IP> \
  --token <AGENT_TOKEN> \
  --name node-01
```

会安装：
- `/usr/local/bin/gpanel-agent`
- `/etc/gpanel/agent.env`
- `gpanel-agent.service`

Agent 负责：
- 心跳上报
- 拉取任务
- 执行 `gost.install / gost.apply_forward / gost.apply_tunnel / gost.start / gost.stop / gost.restart / gost.status`

---

## 当前推荐测试路径

### 1. 部署控制面
在 A 机器安装 panel。

### 2. 接入至少 1~2 个节点
在 B / C 机器安装 host-agent。

### 3. 在 Web UI 中验证
优先测试：
- Nodes
- Forwards
- Tunnels
- Chains
- Runtime

### 4. 建议先验证的功能
- 单节点 TCP/UDP 转发
- 单节点 HTTP/SOCKS5 隧道
- 简单链路：`B -> C`
- 多跳链路：`B -> C -> D`

---

## 当前限制（实话）

以下能力仍在继续完善中：
- Runtime 的完整 desired vs actual 对账
- 链路编排更精细的端口/目标映射策略
- 更完整的 agent 运行态回采
- 真正多机环境下的完整闭环验证

所以当前版本适合：
- 持续开发
- 提前部署
- 小范围联调

但如果你要说“已经完全生产就绪”，那还不够。

---

## 安全建议

首次安装后务必修改：
- `POSTGRES_PASSWORD`
- `JWT_SECRET`
- `AGENT_TOKEN`
- `ADMIN_PASSWORD`

并建议：
- 使用 HTTPS
- 限制面板来源 IP
- 将 agent token 视为敏感凭据
