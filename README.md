# GPanel

一个用于管理多节点的面板（Panel）+ Agent 项目。

## 快速安装（推荐）
```bash
curl -fsSL https://raw.githubusercontent.com/supernaga/gpanel/main/deploy/install-panel.sh | bash
安装后访问：http://服务器IP

节点接入

shell

curl -fsSL https://raw.githubusercontent.com/supernaga/gpanel/main/deploy/install-agent.sh | bash -s -- \
--panel http://<PANEL_IP>:8080 \
--token <AGENT_TOKEN> \
--name node-01
Docker 手动安装

shell

cd deploy
docker compose up -d --build
安全建议（强烈）

• 首次安装后立即修改：
• POSTGRES_PASSWORD
• JWT_SECRET
• AGENT_TOKEN
• 不要长期使用默认密码。

