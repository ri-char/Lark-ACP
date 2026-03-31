# Lark-ACP

连接飞书和ACP(Agent Communication Protocol)的工具。让你能够在飞书中使用OpenCode、Claude、Codex等任何支持ACP的工具。

## 配置

配置文件位于 `~/.config/lark-acp/config.toml`：

```toml
feishu_app_id = "clxxxxxxxxxd"
feishu_app_secret = "Zxxxxxxxxxxgxez"
feishu_user_id = "ou_333axxxxxxxxxf820298" # openid
default_agent = "opencode"

[[agent]]
id = "opencode"
cmd = ["opencode", "acp"]

[[agent]]
id = "claude"
cmd = ["claude-code-acp"]
```

会话信息存储在 `~/.config/lark-acp/session.json`。

## 使用

1. 在飞书开放平台创建应用，配置：
   - 机器人菜单事件（event_key: `new_session`）
   - 启用长连接（WebSocket）接收事件

2. 运行程序：

```bash
./lark-acp
```

3. 在飞书中点击机器人菜单的"新建会话"按钮

4. 选择 Agent → 输入路径 → 创建会话

5. 程序会创建飞书群，将用户加入并设置为管理员

6. 在群中发送消息与 Agent 对话

## 功能流程

### 新建会话流程

```
用户点击菜单 → 选择Agent卡片 → 选择Agent → 输入路径卡片 → 输入路径 → 创建ACP Session → 创建飞书群 → 添加用户为管理员
```

## 功能特性

- 纯 WebSocket 长连接：通过飞书 WebSocket 接收所有事件，无需公网 IP、无需 HTTP 服务
- 交互式卡片
- 支持多 Agent 配置
- 支持权限选择卡片