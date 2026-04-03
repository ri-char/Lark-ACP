package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/ri-char/lark-acp/logger"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkapplication "github.com/larksuite/oapi-sdk-go/v3/service/application/v6"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"

	"github.com/ri-char/lark-acp/acp"
	"github.com/ri-char/lark-acp/config"
	"github.com/ri-char/lark-acp/feishu"
	"github.com/ri-char/lark-acp/session"
)

func main() {
	// Init logger
	logger.Init(slog.LevelDebug)
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}
	acp.InitACPClientManager(cfg.Agents)
	logger.Info("✅ Configuration loaded")

	// Create cancelable context for Store and WebSocket client
	ctx, cancel := context.WithCancel(context.Background())

	// Load session store
	err = session.InitStore(ctx)
	if err != nil {
		logger.Fatalf("Failed to load session store: %v", err)
	}
	logger.Info("✅ Session store loaded")

	// Initialize Feishu client for API calls
	feishu.Init(cfg.FeishuAppID, cfg.FeishuAppSecret)

	// Create event dispatcher for WebSocket events
	eventHandler := dispatcher.NewEventDispatcher(cfg.FeishuVerificationToken, cfg.FeishuEventEncryptKey).
		OnP2BotMenuV6(handleBotMenu).
		OnP2MessageReceiveV1(handleMessageReceive).
		OnP2CardActionTrigger(handleCardActionTrigger).
		OnP2ChatDisbandedV1(handleChatDisband)

	// Create WebSocket client for long connection
	cli := larkws.NewClient(cfg.FeishuAppID, cfg.FeishuAppSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogger(logger.NewLarkLogger(slog.LevelInfo)),
	)

	// Start WebSocket client in goroutine
	websocketQuit := make(chan struct{}, 1)
	go func() {
		logger.Info("🔌 Connecting to Feishu WebSocket...")
		if err := cli.Start(ctx); err != nil {
			logger.Warnf("WebSocket client stopped: %v", err)
			websocketQuit <- struct{}{}
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Waiting
	select {
	case <-quit:
	case <-websocketQuit:
	}

	cancel()
	acp.ACPClientManagerInstance.CloseAll()
}

func handleChatDisband(ctx context.Context, event *larkim.P2ChatDisbandedV1) error {
	chatIdPtr := event.Event.ChatId
	if chatIdPtr == nil {
		logger.Debugf("Chat disbanded event missing chat ID")
		return nil
	}
	chatId := *chatIdPtr
	logger.Debugf("Chat disbanded: %s", chatId)

	// 查找对应的 session信息
	sessionInfo, ok := session.SessionStoreInstance.Get(chatId)
	if !ok {
		logger.Debugf("No session info found for chat: %s", chatId)
		return nil
	}
	// 关闭对应的 ACP agent
	acp.ACPClientManagerInstance.CloseAgent(chatId, sessionInfo.ACPSessionID)
	if err := session.SessionStoreInstance.Delete(chatId); err != nil {
		logger.Warnf("Failed to delete session info for chat: %s, error: %v", chatId, err)
	}
	return nil
}

// handleBotMenu handles bot menu events (new_session)
func handleBotMenu(ctx context.Context, event *larkapplication.P2BotMenuV6) error {
	if event.Event == nil {
		return nil
	}

	var eventKey string
	if event.Event.EventKey != nil {
		eventKey = *event.Event.EventKey
	}

	var openID string
	if event.Event.Operator != nil && event.Event.Operator.OperatorId != nil && event.Event.Operator.OperatorId.OpenId != nil {
		openID = *event.Event.Operator.OperatorId.OpenId
	}
	logger.Debugf("Bot menu event: key=%s, operator=%s", eventKey, openID)

	switch eventKey {
	case "new_session":
		go handleNewSession(ctx, openID)
	case "load_session":
		go handleLoadSession(ctx, openID)
	}

	return nil
}

// handleMessageReceive handles message receive events
func handleMessageReceive(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	if event.Event == nil || event.Event.Message == nil {
		return nil
	}

	var chatID string
	if event.Event.Message.ChatId != nil {
		chatID = *event.Event.Message.ChatId
	}

	var openID string
	if event.Event.Sender != nil && event.Event.Sender.SenderId != nil && event.Event.Sender.SenderId.OpenId != nil {
		openID = *event.Event.Sender.SenderId.OpenId
	}

	var content string
	if event.Event.Message.Content != nil {
		content = *event.Event.Message.Content
	}

	var msgType string
	if event.Event.Message.MessageType != nil {
		msgType = *event.Event.Message.MessageType
	}
	parentId := event.Event.Message.ParentId
	msgId := event.Event.Message.MessageId
	// event.Event.Message.Mentions
	logger.Debug("Message event", "chat", chatID, "sender", openID, "type", msgType, "content", content, "parentId", parentId)

	go handleMessage(ctx, msgId, chatID, msgType, content)
	return nil
}

// handleMessage handles messages from Feishu groups
func handleMessage(ctx context.Context, msgId *string, chatID, msgType, content string) error {
	info, ok := session.SessionStoreInstance.Get(chatID)
	if !ok {
		logger.Warnf("No session found for chat: %s", chatID)
		return nil
	}
	agent, ok := getOrRecoveryAgentBySessionInfo(ctx, info)
	if !ok {
		logger.Warnf("No agent found for chat: %s", chatID)
		feishu.SendMessage(ctx, chatID, "会话已过期或无法恢复")
		return nil
	}
	prompt, err := feishu.FeishuMsgToPrompt(ctx, &info.UserMsg, agent.PromptImage, msgId, msgType, content)
	if err != nil {
		feishu.SendMessage(ctx, chatID, "解析失败："+err.Error())
	}
	if len(prompt) > 0 {
		if err := agent.SendMessage(ctx, info.ACPSessionID, prompt); err != nil {
			logger.Warnf("Failed to send message to ACP: %v", err)
			return err
		}
	}
	info.Mu.Lock()
	defer info.Mu.Unlock()
	info.CloseStreamCard()
	return nil
}

// handleCardActionTrigger handles card action trigger events
func handleCardActionTrigger(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
	if event.Event == nil || event.Event.Action == nil {
		return nil, nil
	}

	openID := event.Event.Operator.OpenID
	action := event.Event.Action

	logger.Debugf("Card action trigger from %s: action.Name=%s", openID, action.Name)

	// 根据按钮 name 判断操作类型
	buttonName := action.Name

	switch buttonName {
	case "new_session_form":
		// 从 form_value 获取表单数据
		formValue := action.FormValue
		if formValue == nil {
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{
					Type:    "error",
					Content: "表单数据无效",
				},
			}, nil
		}

		// 获取选择的 agent
		agentName := ""
		if v, ok := formValue["agent_type"].(string); ok {
			agentName = v
		}

		// 获取输入的路径
		path := ""
		if v, ok := formValue["path_input"].(string); ok {
			path = v
		}

		if path == "" {
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{
					Type:    "error",
					Content: "请输入路径",
				},
			}, nil
		}

		if agentName == "" {
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{
					Type:    "error",
					Content: "请选择 Agent",
				},
			}, nil
		}

		if !filepath.IsAbs(path) {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return nil, fmt.Errorf("invalid path: %w", err)
			}
			path = absPath
		}

		go createSession(openID, agentName, path, event.Event.Context.OpenMessageID)
		return &callback.CardActionTriggerResponse{
			Card: &callback.Card{
				Type: "raw",
				Data: feishu.AgentSelectionFreezeCard(agentName, path),
			},
		}, nil
	case "load_session_agent":
		// 从 form_value 获取表单数据
		formValue := action.FormValue
		if formValue == nil {
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{
					Type:    "error",
					Content: "表单数据无效",
				},
			}, nil
		}

		// 获取选择的 agent
		agentName := ""
		if v, ok := formValue["agent_type"].(string); ok {
			agentName = v
		}

		if agentName == "" {
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{
					Type:    "error",
					Content: "请选择 Agent",
				},
			}, nil
		}
		msgId := event.Event.Context.OpenMessageID
		go handleLoadSessionStage1(ctx, msgId, agentName)

		return &callback.CardActionTriggerResponse{}, nil
	case "load_session_session":
		formValue := action.FormValue
		if formValue == nil {
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{
					Type:    "error",
					Content: "表单数据无效",
				},
			}, nil
		}
		otherValue := action.Value
		if otherValue == nil {
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{
					Type:    "error",
					Content: "表单数据无效",
				},
			}, nil
		}
		agentId, ok := otherValue["agent_id"].(string)
		if !ok {
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{
					Type:    "error",
					Content: "表单数据无效",
				},
			}, nil
		}

		sessionID := ""
		if v, ok := formValue["session_id"].(string); ok {
			sessionID = v
		}

		if sessionID == "" {
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{
					Type:    "error",
					Content: "请选择会话",
				},
			}, nil
		}
		msgId := event.Event.Context.OpenMessageID

		go handleLoadSessionStage2(ctx, openID, msgId, agentId, sessionID)
		return &callback.CardActionTriggerResponse{
			Card: &callback.Card{
				Type: "raw",
				Data: feishu.LoadSessionAgentSessionFreezeCard(agentId),
			},
		}, nil
	}

	// 处理权限选择
	actionType, ok := action.Value["action"]
	if !ok {
		return nil, nil
	}

	if actionType == "permission" {
		value := action.Value
		requestID, _ := value["request_id"].(string)
		optionID, _ := value["option_id"].(string)
		cancel, _ := value["cancel"].(bool)
		pending, ok := session.GetPermissionManager().Get(requestID)
		if !ok {
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{
					Type:    "error",
					Content: "权限请求已过期",
				},
			}, nil
		}
		if cancel {
			pending.ToolCard.CancelPermission()
			pending.Response <- session.PermissionResponse{
				Cancelled: true,
			}
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{
					Type:    "info",
					Content: "已取消权限请求",
				},
				Card: &callback.Card{
					Type: "raw",
					Data: pending.ToolCard.CetCardStructure(),
				},
			}, nil
		} else {
			pending.ToolCard.SelectPermission(optionID)
			pending.Response <- session.PermissionResponse{
				OptionId: acpsdk.PermissionOptionId(optionID),
			}

			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{
					Type:    "success",
					Content: "已处理权限请求",
				},
				Card: &callback.Card{
					Type: "raw",
					Data: pending.ToolCard.CetCardStructure(),
				},
			}, nil
		}
	}

	if actionType == "change_model" {
		chatId := event.Event.Context.OpenChatID

		sessionInfo, ok := session.SessionStoreInstance.Get(chatId)
		if !ok {
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{
					Type:    "error",
					Content: "会话不存在",
				},
			}, nil
		}
		go handleChangeModel(ctx, sessionInfo, action.Option)
		return &callback.CardActionTriggerResponse{}, nil
	}
	if actionType == "change_mode" {
		sessionInfo, ok := session.SessionStoreInstance.Get(event.Event.Context.OpenChatID)
		if !ok {
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{
					Type:    "error",
					Content: "会话不存在",
				},
			}, nil
		}
		go handleChangeMode(ctx, sessionInfo, action.Option)
		return &callback.CardActionTriggerResponse{}, nil

	}
	return nil, nil
}

// handleNewSession initiates the session creation flow
func handleNewSession(ctx context.Context, openID string) {
	logger.Debugf("New session request from: %s", openID)

	agentNames := acp.ACPClientManagerInstance.GetAllAgentNames()
	if len(agentNames) == 0 {
		logger.Debugf("no agents configured")
		return
	}

	cardContent := feishu.AgentSelectionCard(agentNames)
	if err := feishu.SendInteractiveCardToUser(ctx, openID, cardContent); err != nil {
		logger.Warnf("failed to send agent selection card: %v", err)
		return
	}

	// logger.Debugf("Agent selection card sent to: %s", openID)
}

func handleLoadSession(ctx context.Context, openID string) {
	logger.Debugf("Load session request from: %s", openID)
	agentNames := acp.ACPClientManagerInstance.GetAllAgentNames()
	cardContent := feishu.LoadSessionAgentSelectionCard(agentNames)
	if err := feishu.SendInteractiveCardToUser(ctx, openID, cardContent); err != nil {
		logger.Warnf("failed to send agent selection card: %v", err)
		return
	}
	// logger.Debugf("Load session agent selection card sent to: %s", openID)
}

func handleLoadSessionStage1(ctx context.Context, msgId, agentName string) {
	agentCfg, ok := acp.ACPClientManagerInstance.FindAgentConfigById(agentName)
	if !ok {
		logger.Warnf("Agent %s not found", agentName)
		feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("Agent%s无法找到", agentName)), msgId)
		return
	}

	agent, err := acp.New(agentCfg, nil)
	if err != nil {
		logger.Warnf("Failed to start ACP: %v", err)
		feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("Agent 启动失败：%v", err)), msgId)
		return
	}
	err = agent.Initialize(ctx)
	if err != nil {
		agent.Close()
		logger.Warnf("Failed to initialize ACP: %v", err)
		feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("Agent 初始化失败：%v", err)), msgId)
		return
	}
	sessions, err := agent.ListSessions(ctx)
	if err != nil {
		agent.Close()
		logger.Warnf("Failed to list sessions: %v", err)
		feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("获取会话列表失败：%v", err)), msgId)
		return
	}
	if len(sessions) == 0 {
		agent.Close()
		logger.Warnf("No sessions found for agent: %s", agentName)
		feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", "没有可用的会话"), msgId)
		return
	}
	cardContent := feishu.LoadSessionAgentSessionCard(sessions, agentName)
	if err := feishu.UpdateInteractiveCard(ctx, cardContent, msgId); err != nil {
		logger.Warnf("Failed to send session selection card: %v", err)
	}
	agent.Close()
}

func handleLoadSessionStage2(ctx context.Context, openID, msgId, agentName, sessionID string) {
	existed_session, ok := session.SessionStoreInstance.GetByACPSession(agentName, sessionID)
	if ok {
		chatId := existed_session.FeishuChatID
		shareUrlResp, err := feishu.GetGroupShareLink(ctx, chatId)
		if err != nil {
			logger.Warnf("Failed to get share link for existing session: %v", err)
			feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("获取现有群链接失败：%v", err)), msgId)
			return
		}
		if shareUrlResp.Success() && shareUrlResp.Data != nil && shareUrlResp.Data.ShareLink != nil {
			shareUrl := *shareUrlResp.Data.ShareLink
			feishu.UpdateInteractiveCard(ctx, feishu.NewSessionFinishCard(agentName, existed_session.Path, shareUrl, "恢复会话 - 会话已存在"), msgId)
			return
		}
		if shareUrlResp.Code == 232019 || shareUrlResp.Code == 232065 {
			logger.Warnf("Failed to get share link for existing session: %v", err)
			feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("获取现有群链接失败：%v", shareUrlResp.Msg)), msgId)
			return
		}
	}
	agentCfg, ok := acp.ACPClientManagerInstance.FindAgentConfigById(agentName)
	if !ok {
		logger.Warnf("Agent %s not found", agentName)
		feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("Agent%s无法找到", agentName)), msgId)
		return
	}

	agent, err := acp.New(agentCfg, nil)
	if err != nil {
		logger.Warnf("Failed to start ACP: %v", err)
		feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("Agent 启动失败：%v", err)), msgId)
		return
	}
	err = agent.Initialize(ctx)
	if err != nil {
		agent.Close()
		logger.Warnf("Failed to initialize ACP: %v", err)
		feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("Agent 初始化失败：%v", err)), msgId)
		return
	}
	path := ""
	sessions, err := agent.ListSessions(ctx)
	if err != nil {
		agent.Close()
		logger.Warnf("Failed to list sessions: %v", err)
		feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("获取会话列表失败：%v", err)), msgId)
		return
	}
	found := false
	for _, s := range sessions {
		if string(s.SessionId) == sessionID {
			path = s.Cwd
			found = true
			break
		}
	}
	if !found {
		agent.Close()
		logger.Warnf("Session %s not found for agent: %s", sessionID, agentName)
		feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", "会话未找到"), msgId)
		return
	}
	models, modes, err := agent.LoadSession(ctx, sessionID, path)
	if err != nil {
		agent.Close()
		logger.Warnf("Failed to create ACP session: %v", err)
		feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("恢复会话失败：%v", err)), msgId)
		return
	}

	groupChatID, err := feishu.CreateGroup(ctx, fmt.Sprintf("%s: %s", agentName, filepath.Base(path)), openID)
	if err != nil {
		agent.Close()
		logger.Warnf("Failed to create group: %v", err)
		feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("创建群组失败：%v", err)), msgId)
		return
	}

	acp.ACPClientManagerInstance.Set(groupChatID, agent)
	sessionInfo := session.Session{
		FeishuChatID: groupChatID,
		ACPSessionID: sessionID,
		AgentName:    agentName,
		Path:         path,
		Models:       models.AvailableModels,
		Modes:        modes.AvailableModes,
		ModelId:      string(models.CurrentModelId),
		ModeId:       string(modes.CurrentModeId),
	}
	if err := session.SessionStoreInstance.Set(groupChatID, &sessionInfo); err != nil {
		logger.Warnf("failed to save session: %v", err)
	}
	agent.SetSessionChatID(&sessionInfo)
	shareLinkResp, err := feishu.GetGroupShareLink(ctx, groupChatID)
	if err != nil || !shareLinkResp.Success() || shareLinkResp.Data.ShareLink == nil {
		logger.Warnf("Failed to get group share link: %v", err)
		feishu.UpdateInteractiveCard(ctx, feishu.NewSessionFinishCard(agentName, path, "", "会话已恢复（获取分享链接失败）"), msgId)
		return
	}
	feishu.UpdateInteractiveCard(ctx, feishu.NewSessionFinishCard(agentName, path, *shareLinkResp.Data.ShareLink, "会话已恢复"), msgId)
	sessionInfo.UpdateInformationCardToFeishu(ctx)

}

func handleChangeModel(ctx context.Context, sessionInfo *session.Session, modelId string) {
	agent, ok := getOrRecoveryAgentBySessionInfo(ctx, sessionInfo)
	if !ok {
		feishu.SendMessage(ctx, sessionInfo.FeishuChatID, "会话已过期或无法恢复")
		return
	}
	err := agent.SetModel(ctx, sessionInfo.ACPSessionID, modelId)
	if err != nil {
		logger.Warnf("Failed to set model: %v", err)
		feishu.SendMessage(ctx, sessionInfo.FeishuChatID, fmt.Sprintf("切换模型失败：%v", err))
		return
	}
	sessionInfo.SetModel(modelId)
	sessionInfo.UpdateInformationCardToFeishu(ctx)
}

func handleChangeMode(ctx context.Context, sessionInfo *session.Session, modeId string) {
	agent, ok := getOrRecoveryAgentBySessionInfo(ctx, sessionInfo)
	if !ok {
		feishu.SendMessage(ctx, sessionInfo.FeishuChatID, "会话已过期或无法恢复")
		return
	}
	err := agent.SetMode(ctx, sessionInfo.ACPSessionID, modeId)
	if err != nil {
		logger.Warnf("Failed to set mode: %v", err)
		feishu.SendMessage(ctx, sessionInfo.FeishuChatID, fmt.Sprintf("切换状态失败：%v", err))
		return
	}
	sessionInfo.SetMode(modeId)
	sessionInfo.UpdateInformationCardToFeishu(ctx)
}

// createSession creates an ACP session and Feishu group
func createSession(openID, agentName, path, msgId string) {
	ctx := context.Background()
	logger.Info("Bot menu create session", "agentName", agentName, "path", path, "open_id", openID)
	agentCfg, ok := acp.ACPClientManagerInstance.FindAgentConfigById(agentName)
	if !ok {
		logger.Warnf("Agent %s not found", agentName)
		feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("创建会话失败", fmt.Sprintf("Agent%s无法找到", agentName)), msgId)
		return
	}

	agent, err := acp.New(agentCfg, nil)
	if err != nil {
		logger.Warnf("Failed to start ACP: %v", err)
		feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("创建会话失败", fmt.Sprintf("启动 Agent 失败：%v", err)), msgId)
		return
	}

	if err := agent.Initialize(ctx); err != nil {
		agent.Close()
		logger.Warnf("Failed to initialize ACP: %v", err)
		feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("创建会话失败", fmt.Sprintf("初始化 Agent 失败：%v", err)), msgId)
		return
	}

	sessionID, models, modes, err := agent.CreateSession(ctx, path)
	if err != nil {
		agent.Close()
		logger.Warnf("Failed to create ACP session: %v", err)
		feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("创建会话失败", fmt.Sprintf("创建会话失败：%v", err)), msgId)
		return
	}

	groupChatID, err := feishu.CreateGroup(ctx, fmt.Sprintf("%s: %s", agentName, filepath.Base(path)), openID)
	if err != nil {
		agent.Close()
		logger.Warnf("Failed to create group: %v", err)
		feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("创建会话失败", fmt.Sprintf("创建群组失败：%v", err)), msgId)
		return
	}

	acp.ACPClientManagerInstance.Set(groupChatID, agent)
	sessionInfo := session.Session{
		FeishuChatID: groupChatID,
		ACPSessionID: sessionID,
		AgentName:    agentName,
		Path:         path,
		Models:       models.AvailableModels,
		Modes:        modes.AvailableModes,
		ModelId:      string(models.CurrentModelId),
		ModeId:       string(modes.CurrentModeId),
	}
	if err := session.SessionStoreInstance.Set(groupChatID, &sessionInfo); err != nil {
		logger.Warnf("failed to save session: %v", err)
	}
	agent.SetSessionChatID(&sessionInfo)
	shareLinkResp, err := feishu.GetGroupShareLink(ctx, groupChatID)
	if err != nil || !shareLinkResp.Success() || shareLinkResp.Data.ShareLink == nil {
		logger.Warnf("Failed to get group share link: %v", err)
		feishu.UpdateInteractiveCard(ctx, feishu.NewSessionFinishCard(agentName, path, "", "会话已恢复（获取分享链接失败）"), msgId)
		return
	}
	feishu.UpdateInteractiveCard(ctx, feishu.NewSessionFinishCard(agentName, path, *shareLinkResp.Data.ShareLink, "会话已创建"), msgId)
	sessionInfo.UpdateInformationCardToFeishu(ctx)
}

func getOrRecoveryAgentBySessionInfo(ctx context.Context, info *session.Session) (*acp.Client, bool) {
	agent, ok := acp.ACPClientManagerInstance.Get(info.FeishuChatID)
	if !ok {
		logger.Debugf("No ACP client found for chat: %s, attempting to restore...", info.FeishuChatID)
		agent, ok = restoreAgent(ctx, info)
		if !ok {
			return nil, false
		}
	}
	return agent, ok
}

// restoreAgent restores an ACP client connection for a chat
func restoreAgent(ctx context.Context, info *session.Session) (*acp.Client, bool) {
	agentConfig, ok := acp.ACPClientManagerInstance.FindAgentConfigById(info.AgentName)
	if !ok {
		logger.Warnf("Agent config not found: %s", info.AgentName)
		return nil, false
	}

	newAgent, err := acp.New(agentConfig, nil)
	if err != nil {
		logger.Warnf("Failed to create ACP client: %v", err)
		return nil, false
	}

	if err := newAgent.Initialize(ctx); err != nil {
		newAgent.Close()
		logger.Warnf("Failed to initialize ACP: %v", err)
		return nil, false
	}
	models, modes, err := newAgent.LoadSession(ctx, info.ACPSessionID, info.Path)
	if err != nil {
		newAgent.Close()
		logger.Warnf("Failed to load session: %v", err)
		return nil, false
	}
	modelId := info.GetModel()
	if len(modelId) > 0 {
		err = newAgent.SetModel(ctx, info.ACPSessionID, modelId)
		if err != nil {
			logger.Warnf("Failed to set model: %v", err)
			if models != nil {
				info.SetModel(string(models.CurrentModelId))
			}
		}
	}
	modeId := info.GetMode()
	if len(modeId) > 0 {
		err = newAgent.SetMode(ctx, info.ACPSessionID, modeId)
		if err != nil {
			logger.Warnf("Failed to set mode: %v", err)
			if modes != nil {
				info.SetMode(string(modes.CurrentModeId))
			}
		}
	}
	if models != nil {
		info.SetModels(models.AvailableModels)
	}
	if modes != nil {
		info.SetModes(modes.AvailableModes)
	}

	newAgent.SetSessionChatID(info)
	acp.ACPClientManagerInstance.Set(info.FeishuChatID, newAgent)
	info.UpdateInformationCardToFeishu(ctx)
	logger.Debugf("Agent connection restored for chat: %s", info.FeishuChatID)
	return newAgent, true
}
