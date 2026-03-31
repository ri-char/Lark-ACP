package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	acpsdk "github.com/coder/acp-go-sdk"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
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

type App struct {
	cfg           *config.Config
	store         *session.SessionStore
	feishu        *feishu.Client
	agents        map[string]*acp.Client // chatID -> ACP client
	sessionToChat map[string]string      // sessionID -> chatID
	permissionMgr *session.PermissionManager
}

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Load session store
	store, err := session.NewStore()
	if err != nil {
		log.Fatalf("Failed to load session store: %v", err)
	}

	// Initialize Feishu client for API calls
	fs, err := feishu.New(cfg.FeishuAppID, cfg.FeishuAppSecret)
	if err != nil {
		log.Fatalf("Failed to create Feishu client: %v", err)
	}

	app := &App{
		cfg:           cfg,
		store:         store,
		feishu:        fs,
		agents:        make(map[string]*acp.Client),
		sessionToChat: make(map[string]string),
		permissionMgr: session.NewPermissionManager(),
	}

	// Create event dispatcher for WebSocket events
	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnP2BotMenuV6(app.handleBotMenu).
		OnP2MessageReceiveV1(app.handleMessageReceive).
		OnP2CardActionTrigger(app.handleCardActionTrigger).
		OnP2ChatDisbandedV1(app.handleChatDisband)

	// Create WebSocket client for long connection
	cli := larkws.NewClient(cfg.FeishuAppID, cfg.FeishuAppSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkcore.LogLevelDebug),
	)

	// Create cancelable context for WebSocket client
	ctx, cancel := context.WithCancel(context.Background())

	// Start WebSocket client in goroutine
	go func() {
		log.Println("Starting Feishu WebSocket client...")
		if err := cli.Start(ctx); err != nil {
			log.Printf("WebSocket client stopped: %v", err)
		}
	}()

	log.Println("Lark-ACP server started (WebSocket mode)")
	log.Println("Waiting for events...")

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Cleanup
	log.Println("Shutting down...")
	cancel()
	for _, agent := range app.agents {
		agent.Close()
	}
}

func (app *App) handleChatDisband(ctx context.Context, event *larkim.P2ChatDisbandedV1) error {
	chatIdPtr := event.Event.ChatId
	if chatIdPtr == nil {
		log.Printf("Chat disbanded event missing chat ID")
		return nil
	}
	chatId := *chatIdPtr
	log.Printf("Chat disbanded: %s", chatId)

	// 查找对应的 session信息
	sessionInfo, ok := app.store.Get(chatId)
	if !ok {
		log.Printf("No session info found for chat: %s", chatId)
		return nil
	}
	// 关闭对应的 ACP agent
	agent, ok := app.agents[chatId]
	if ok {
		delete(app.agents, chatId)
		agentInUse := false
		for _, a := range app.agents {
			if a == agent {
				agentInUse = true
				break
			}
		}
		if agentInUse {
			log.Printf("Agent for session %s is still in use by another chat, not closing", sessionInfo.ACPSessionID)
		} else {
			log.Printf("Closing agent for session %s", sessionInfo.ACPSessionID)
			agent.Close()
		}
	}
	// 删除 session信息
	delete(app.sessionToChat, sessionInfo.ACPSessionID)
	if err := app.store.Delete(chatId); err != nil {
		log.Printf("Failed to delete session info for chat: %s, error: %v", chatId, err)
	} else {
		log.Printf("Deleted session info for chat: %s", chatId)
		app.store.Save()
	}
	return nil
}

// handleBotMenu handles bot menu events (new_session)
func (app *App) handleBotMenu(ctx context.Context, event *larkapplication.P2BotMenuV6) error {
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
	log.Printf("Bot menu event: key=%s, operator=%s", eventKey, openID)

	switch eventKey {
	case "new_session":
		go app.handleNewSession(ctx, openID)
	case "load_session":
		go app.handleLoadSession(ctx, openID)
	}

	return nil
}

// handleMessageReceive handles message receive events
func (app *App) handleMessageReceive(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
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

	if msgType == "text" {
		var textContent struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(content), &textContent); err == nil {
			content = textContent.Text
		}
	}

	log.Printf("Message event: chat=%s, sender=%s, type=%s, content=%s", chatID, openID, msgType, content)

	go app.handleMessage(ctx, chatID, openID, content)
	return nil
}

// handleCardActionTrigger handles card action trigger events
func (app *App) handleCardActionTrigger(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
	if event.Event == nil || event.Event.Action == nil {
		return nil, nil
	}

	openID := event.Event.Operator.OpenID
	action := event.Event.Action

	log.Printf("Card action trigger from %s: action.Name=%s action=%v", openID, action.Name, action)

	// 根据按钮 name 判断操作类型
	buttonName := action.Name

	switch buttonName {
	case "new_session_form":
		// 从 form_value 获取表单数据
		formValue := action.FormValue
		log.Printf("formValue: %v", formValue)
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

		go app.createSession(openID, agentName, path, event.Event.Context.OpenMessageID)
		return &callback.CardActionTriggerResponse{
			Card: &callback.Card{
				Type: "raw",
				Data: feishu.AgentSelectionFreezeCard(agentName, path),
			},
		}, nil
	case "load_session_agent":
		// 从 form_value 获取表单数据
		formValue := action.FormValue
		log.Printf("formValue: %v", formValue)
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
		go app.handleLoadSessionStage1(ctx, openID, msgId, agentName)

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
		go app.handleLoadSessionStage2(ctx, openID, msgId, agentId, sessionID)
		return &callback.CardActionTriggerResponse{}, nil
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
		pending, ok := app.permissionMgr.Get(requestID)
		if !ok {
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{
					Type:    "error",
					Content: "权限请求已过期",
				},
				Card: &callback.Card{
					Type: "raw",
					Data: feishu.PermissionFreezeCard(pending.Options, pending.ToolCall, true, ""),
				},
			}, nil
		}
		if cancel {
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
					Data: feishu.PermissionFreezeCard(pending.Options, pending.ToolCall, true, ""),
				},
			}, nil
		} else {
			pending.Response <- session.PermissionResponse{
				OptionId: acpsdk.PermissionOptionId(optionID),
			}
		}
		return &callback.CardActionTriggerResponse{
			Toast: &callback.Toast{
				Type:    "success",
				Content: "已处理权限请求",
			},
			Card: &callback.Card{
				Type: "raw",
				Data: feishu.PermissionFreezeCard(pending.Options, pending.ToolCall, false, optionID),
			},
		}, nil
	}

	if actionType == "change_model" {
		chatId := event.Event.Context.OpenChatID

		sessionInfo, ok := app.store.Get(chatId)
		if !ok {
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{
					Type:    "error",
					Content: "会话不存在",
				},
			}, nil
		}
		go app.handleChangeModel(ctx, sessionInfo, action.Option)
		return &callback.CardActionTriggerResponse{}, nil
	}
	if actionType == "change_mode" {
		sessionInfo, ok := app.store.Get(event.Event.Context.OpenChatID)
		if !ok {
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{
					Type:    "error",
					Content: "会话不存在",
				},
			}, nil
		}
		go app.handleChangeMode(ctx, sessionInfo, action.Option)
		return &callback.CardActionTriggerResponse{}, nil

	}
	return nil, nil
}

// handleNewSession initiates the session creation flow
func (app *App) handleNewSession(ctx context.Context, openID string) {
	log.Printf("New session request from: %s", openID)

	agentNames := make([]string, 0, len(app.cfg.Agents))
	for _, agents := range app.cfg.Agents {
		agentNames = append(agentNames, agents.Id)
	}

	if len(agentNames) == 0 {
		log.Printf("no agents configured")
		return
	}

	cardContent := feishu.AgentSelectionCard(agentNames)
	if err := app.feishu.SendInteractiveCardToUser(ctx, openID, cardContent); err != nil {
		log.Printf("failed to send agent selection card: %v", err)
		return
	}

	log.Printf("Agent selection card sent to: %s", openID)
}

func (app *App) handleLoadSession(ctx context.Context, openID string) {
	log.Printf("Load session request from: %s", openID)
	agentNames := make([]string, 0, len(app.cfg.Agents))
	for _, agents := range app.cfg.Agents {
		agentNames = append(agentNames, agents.Id)
	}
	cardContent := feishu.LoadSessionAgentSelectionCard(agentNames)
	if err := app.feishu.SendInteractiveCardToUser(ctx, openID, cardContent); err != nil {
		log.Printf("failed to send agent selection card: %v", err)
		return
	}
	log.Printf("Load session agent selection card sent to: %s", openID)
}

func (app *App) handleLoadSessionStage1(ctx context.Context, openID, msgId, agentName string) {
	agentCfg, ok := app.cfg.FindAgentById(agentName)
	if !ok {
		log.Printf("Agent %s not found", agentName)
		app.feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("Agent%s无法找到", agentName)), msgId)
		return
	}

	agent, err := acp.New(agentCfg.Cmd, app.feishu, app.permissionMgr)
	if err != nil {
		log.Printf("Failed to start ACP: %v", err)
		app.feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("Agent启动失败：%v", err)), msgId)
		return
	}
	err = agent.Initialize(ctx)
	if err != nil {
		agent.Close()
		log.Printf("Failed to initialize ACP: %v", err)
		app.feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("Agent初始化失败：%v", err)), msgId)
		return
	}
	sessions, err := agent.ListSessions(ctx)
	if err != nil {
		agent.Close()
		log.Printf("Failed to list sessions: %v", err)
		app.feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("获取会话列表失败：%v", err)), msgId)
		return
	}
	if len(sessions) == 0 {
		agent.Close()
		log.Printf("No sessions found for agent: %s", agentName)
		app.feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", "没有可用的会话"), msgId)
		return
	}
	cardContent := feishu.LoadSessionAgentSessionCard(sessions, agentName)
	if err := app.feishu.UpdateInteractiveCard(ctx, cardContent, msgId); err != nil {
		log.Printf("Failed to send session selection card: %v", err)
	}
	log.Printf("Session selection card sent to: %s", openID)
	agent.Close()
}

func (app *App) handleLoadSessionStage2(ctx context.Context, openID, msgId, agentName, sessionID string) {
	existed_session, ok := app.store.GetByACPSession(agentName, sessionID)
	if ok {
		chatId := existed_session.FeishuChatID
		shareUrlResp, err := app.feishu.GetGroupShareLink(ctx, chatId)
		if err != nil {
			log.Printf("Failed to get share link for existing session: %v", err)
			app.feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("获取现有群链接失败：%v", err)), msgId)
			return
		}
		if shareUrlResp.Success() && shareUrlResp.Data != nil && shareUrlResp.Data.ShareLink != nil {
			shareUrl := *shareUrlResp.Data.ShareLink
			app.feishu.UpdateInteractiveCard(ctx, feishu.NewSessionFinishCard(agentName, existed_session.Path, shareUrl, "恢复会话 - 会话已存在"), msgId)
			return
		}
		if shareUrlResp.Code == 232019 || shareUrlResp.Code == 232065 {
			log.Printf("Failed to get share link for existing session: %v", err)
			app.feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("获取现有群链接失败：%v", shareUrlResp.Msg)), msgId)
			return
		}
	}
	agentCfg, ok := app.cfg.FindAgentById(agentName)
	if !ok {
		log.Printf("Agent %s not found", agentName)
		app.feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("Agent%s无法找到", agentName)), msgId)
		return
	}

	agent, err := acp.New(agentCfg.Cmd, app.feishu, app.permissionMgr)
	if err != nil {
		log.Printf("Failed to start ACP: %v", err)
		app.feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("Agent启动失败：%v", err)), msgId)
		return
	}
	err = agent.Initialize(ctx)
	if err != nil {
		agent.Close()
		log.Printf("Failed to initialize ACP: %v", err)
		app.feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("Agent初始化失败：%v", err)), msgId)
		return
	}
	path := ""
	sessions, err := agent.ListSessions(ctx)
	if err != nil {
		agent.Close()
		log.Printf("Failed to list sessions: %v", err)
		app.feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("获取会话列表失败：%v", err)), msgId)
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
		log.Printf("Session %s not found for agent: %s", sessionID, agentName)
		app.feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", "会话未找到"), msgId)
		return
	}
	models, modes, err := agent.LoadSession(ctx, sessionID, path)
	if err != nil {
		agent.Close()
		log.Printf("Failed to create ACP session: %v", err)
		app.feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("恢复会话失败: %v", err)), msgId)
		return
	}

	groupChatID, err := app.feishu.CreateGroup(ctx, fmt.Sprintf("%s: %s", agentName, filepath.Base(path)), openID)
	if err != nil {
		agent.Close()
		log.Printf("Failed to create group: %v", err)
		app.feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("加载会话失败", fmt.Sprintf("创建群组失败: %v", err)), msgId)
		return
	}

	app.agents[groupChatID] = agent
	app.sessionToChat[sessionID] = groupChatID
	sessionInfo := session.SessionInfo{
		FeishuChatID:     groupChatID,
		ACPSessionID:     sessionID,
		AgentName:        agentName,
		Path:             path,
		ToolCallIdToInfo: make(map[string]*session.ToolCallIdInfo),
		Models:           models,
		Modes:            modes,
	}
	if err := app.store.Set(groupChatID, &sessionInfo); err != nil {
		log.Printf("Warning: failed to save session: %v", err)
	}
	agent.SetSessionChatID(&sessionInfo)
	shareLinkResp, err := app.feishu.GetGroupShareLink(ctx, groupChatID)
	if err != nil || !shareLinkResp.Success() || shareLinkResp.Data.ShareLink == nil {
		log.Printf("Failed to get group share link: %v", err)
		app.feishu.UpdateInteractiveCard(ctx, feishu.NewSessionFinishCard(agentName, path, "", "会话已恢复（获取分享链接失败）"), msgId)
		return
	}
	app.feishu.UpdateInteractiveCard(ctx, feishu.NewSessionFinishCard(agentName, path, *shareLinkResp.Data.ShareLink, "会话已恢复"), msgId)
	app.feishu.SendOrUpdatePinCard(ctx, &sessionInfo)
	app.store.Save()

}

func (app *App) handleChangeModel(ctx context.Context, sessionInfo *session.SessionInfo, modelId string) {
	agent, ok := app.getOrRecoveryAgentBySessionInfo(ctx, sessionInfo)
	if !ok {
		app.feishu.SendMessage(ctx, sessionInfo.FeishuChatID, "会话已过期或无法恢复")
		return
	}
	err := agent.SetModel(ctx, sessionInfo.ACPSessionID, modelId)
	if err != nil {
		log.Printf("Failed to set model: %v", err)
		app.feishu.SendMessage(ctx, sessionInfo.FeishuChatID, fmt.Sprintf("切换模型失败: %v", err))
		return
	}
	sessionInfo.LastModelId = modelId
	if sessionInfo.Models != nil {
		sessionInfo.Models.CurrentModelId = acpsdk.ModelId(modelId)
	}
	app.store.Save()
	app.feishu.SendOrUpdatePinCard(ctx, sessionInfo)
}

func (app *App) handleChangeMode(ctx context.Context, sessionInfo *session.SessionInfo, modeId string) {
	agent, ok := app.getOrRecoveryAgentBySessionInfo(ctx, sessionInfo)
	if !ok {
		app.feishu.SendMessage(ctx, sessionInfo.FeishuChatID, "会话已过期或无法恢复")
		return
	}
	err := agent.SetMode(ctx, sessionInfo.ACPSessionID, modeId)
	if err != nil {
		log.Printf("Failed to set mode: %v", err)
		app.feishu.SendMessage(ctx, sessionInfo.FeishuChatID, fmt.Sprintf("切换状态失败: %v", err))
		return
	}
	sessionInfo.LastModeId = modeId
	if sessionInfo.Modes != nil {
		sessionInfo.Modes.CurrentModeId = acpsdk.SessionModeId(modeId)
	}
	app.store.Save()
	app.feishu.SendOrUpdatePinCard(ctx, sessionInfo)
}

// createSession creates an ACP session and Feishu group
func (app *App) createSession(openID, agentName, path, msgId string) {
	ctx := context.Background()
	agentCfg, ok := app.cfg.FindAgentById(agentName)
	if !ok {
		log.Printf("Agent %s not found", agentName)
		app.feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("创建会话失败", fmt.Sprintf("Agent%s无法找到", agentName)), msgId)
		return
	}

	agent, err := acp.New(agentCfg.Cmd, app.feishu, app.permissionMgr)
	if err != nil {
		log.Printf("Failed to start ACP: %v", err)
		app.feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("创建会话失败", fmt.Sprintf("启动Agent失败: %v", err)), msgId)
		return
	}

	if err := agent.Initialize(ctx); err != nil {
		agent.Close()
		log.Printf("Failed to initialize ACP: %v", err)
		app.feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("创建会话失败", fmt.Sprintf("初始化Agent失败: %v", err)), msgId)
		return
	}

	sessionID, models, modes, err := agent.CreateSession(ctx, path)
	if err != nil {
		agent.Close()
		log.Printf("Failed to create ACP session: %v", err)
		app.feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("创建会话失败", fmt.Sprintf("创建会话失败: %v", err)), msgId)
		return
	}

	groupChatID, err := app.feishu.CreateGroup(ctx, fmt.Sprintf("%s: %s", agentName, filepath.Base(path)), openID)
	if err != nil {
		agent.Close()
		log.Printf("Failed to create group: %v", err)
		app.feishu.UpdateInteractiveCard(ctx, feishu.ErrorCard("创建会话失败", fmt.Sprintf("创建群组失败: %v", err)), msgId)
		return
	}

	app.agents[groupChatID] = agent
	app.sessionToChat[sessionID] = groupChatID
	sessionInfo := session.SessionInfo{
		FeishuChatID:     groupChatID,
		ACPSessionID:     sessionID,
		AgentName:        agentName,
		Path:             path,
		ToolCallIdToInfo: make(map[string]*session.ToolCallIdInfo),
		Models:           models,
		Modes:            modes,
	}
	if err := app.store.Set(groupChatID, &sessionInfo); err != nil {
		log.Printf("Warning: failed to save session: %v", err)
	}
	agent.SetSessionChatID(&sessionInfo)
	shareLinkResp, err := app.feishu.GetGroupShareLink(ctx, groupChatID)
	if err != nil || !shareLinkResp.Success() || shareLinkResp.Data.ShareLink == nil {
		log.Printf("Failed to get group share link: %v", err)
		app.feishu.UpdateInteractiveCard(ctx, feishu.NewSessionFinishCard(agentName, path, "", "会话已恢复（获取分享链接失败）"), msgId)
		return
	}
	app.feishu.UpdateInteractiveCard(ctx, feishu.NewSessionFinishCard(agentName, path, *shareLinkResp.Data.ShareLink, "会话已创建"), msgId)
	app.feishu.SendOrUpdatePinCard(ctx, &sessionInfo)
	app.store.Save()
}

// handleMessage handles messages from Feishu groups
func (app *App) handleMessage(ctx context.Context, chatID, openID, content string) error {
	log.Printf("Message from %s in %s: %s", openID, chatID, content)

	info, ok := app.store.Get(chatID)
	if !ok {
		log.Printf("No session found for chat: %s", chatID)
		return nil
	}
	agent, ok := app.getOrRecoveryAgentBySessionInfo(ctx, info)
	if !ok {
		log.Printf("No agent found for chat: %s", chatID)
		app.feishu.SendMessage(ctx, chatID, "会话已过期或无法恢复")
		return nil
	}

	if err := agent.SendMessage(ctx, info.ACPSessionID, content); err != nil {
		log.Printf("Failed to send message to ACP: %v", err)
		return err
	}
	info.Mu.Lock()
	defer info.Mu.Unlock()
	agent.ResetStreaming(info)
	return nil
}

func (app *App) getOrRecoveryAgentBySessionInfo(ctx context.Context, info *session.SessionInfo) (*acp.Client, bool) {
	agent, ok := app.agents[info.FeishuChatID]
	if !ok {
		log.Printf("No ACP client found for chat: %s, attempting to restore...", info.FeishuChatID)
		agent, ok = app.restoreAgent(ctx, info)
		if !ok {
			return nil, false
		}
	}
	return agent, ok
}

// restoreAgent restores an ACP client connection for a chat
func (app *App) restoreAgent(ctx context.Context, info *session.SessionInfo) (*acp.Client, bool) {
	agentConfig, ok := app.cfg.FindAgentById(info.AgentName)
	if !ok {
		log.Printf("Agent config not found: %s", info.AgentName)
		return nil, false
	}

	newAgent, err := acp.New(agentConfig.Cmd, app.feishu, app.permissionMgr)
	if err != nil {
		log.Printf("Failed to create ACP client: %v", err)
		return nil, false
	}

	if err := newAgent.Initialize(ctx); err != nil {
		newAgent.Close()
		log.Printf("Failed to initialize ACP: %v", err)
		return nil, false
	}
	models, modes, err := newAgent.LoadSession(ctx, info.ACPSessionID, info.Path)
	if err != nil {
		newAgent.Close()
		log.Printf("Failed to load session: %v", err)
		return nil, false
	}
	if len(info.LastModelId) > 0 {
		err = newAgent.SetModel(ctx, info.ACPSessionID, info.LastModelId)
		if err != nil {
			log.Printf("Failed to set model: %v", err)
			if models != nil {
				info.LastModelId = string(models.CurrentModelId)
			}
		} else {
			if models != nil {
				models.CurrentModelId = acpsdk.ModelId(info.LastModelId)
			}
		}
	}
	if len(info.LastModeId) > 0 {
		err = newAgent.SetMode(ctx, info.ACPSessionID, info.LastModeId)
		if err != nil {
			log.Printf("Failed to set mode: %v", err)
			if modes != nil {
				info.LastModeId = string(modes.CurrentModeId)
			}
		} else {
			if modes != nil {
				modes.CurrentModeId = acpsdk.SessionModeId(info.LastModeId)
			}
		}
	}
	info.Models = models
	info.Modes = modes

	newAgent.SetSessionChatID(info)
	app.agents[info.FeishuChatID] = newAgent
	app.feishu.SendOrUpdatePinCard(ctx, info)
	app.store.Save()
	log.Printf("Agent connection restored for chat: %s", info.FeishuChatID)
	return newAgent, true
}
