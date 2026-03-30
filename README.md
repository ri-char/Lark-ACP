# Lark-ACP

连接飞书和ACP(Agent Communication Protocol)的工具。

## 项目结构

```
Lark-ACP/
├── main.go              # 主程序入口
├── config/config.go     # 配置文件解析
├── session/session.go   # 会话存储管理
├── session/creation_state.go # 会话创建状态管理
├── acp/client.go        # ACP客户端 (使用 github.com/coder/acp-go-sdk)
├── feishu/client.go     # 飞书API客户端
├── feishu/handler.go    # 飞书事件处理器
├── feishu/card.go       # 交互式卡片模板
├── feishu/p2p.go        # P2P聊天功能
```

## 配置

配置文件位于 `~/.config/lark-acp/config.toml`：

```toml
feishu_app_id = "clxxxxxxxxxd"
feishu_app_secret = "Zxxxxxxxxxxgxez"
feishu_user_id = "ou_333axxxxxxxxxf820298" # openid
default_agent = "opencode"

[opencode]
cmd = "opencode acp"
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

### 消息转发流程

```
飞书群消息 → ACP Prompt → Agent处理 → SessionUpdate回调 → 飞书群消息
```

## 功能特性

- **纯 WebSocket 长连接**：通过飞书 WebSocket 接收所有事件，无需公网 IP、无需 HTTP 服务
- 处理机器人菜单事件 (`bot.menu_v6`)
- 处理群消息事件 (`im.message.receive_v1`)
- 处理卡片回调事件 (`card.action.trigger`)
- 交互式卡片选择 Agent 和输入路径
- 创建飞书群聊并添加成员和管理员
- 实时转发 ACP 响应到飞书群
- 支持多 Agent 配置