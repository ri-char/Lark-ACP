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
		OnP2CardActionTrigger(app.handleCardActionTrigger)

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
		return app.handleNewSession(ctx, openID)
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

		go app.createSession(openID, agentName, path)
		return &callback.CardActionTriggerResponse{
			Card: &callback.Card{
				Type: "raw",
				Data: feishu.AgentSelectionFreezeCard(agentName, path),
			},
		}, nil
	}

	// 处理权限选择
	
	if actionType, ok := action.Value["action"]; ok && actionType == "permission" {
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

	return nil, nil
}

// handleNewSession initiates the session creation flow
func (app *App) handleNewSession(ctx context.Context, openID string) error {
	log.Printf("New session request from: %s", openID)

	agentNames := make([]string, 0, len(app.cfg.Agents))
	for name := range app.cfg.Agents {
		agentNames = append(agentNames, name)
	}

	if len(agentNames) == 0 {
		return fmt.Errorf("no agents configured")
	}

	cardContent := feishu.AgentSelectionCard(agentNames)
	if err := app.feishu.SendInteractiveCardToUser(ctx, openID, cardContent); err != nil {
		return fmt.Errorf("failed to send agent selection card: %w", err)
	}

	log.Printf("Agent selection card sent to: %s", openID)
	return nil
}

// createSession creates an ACP session and Feishu group
func (app *App) createSession(openID, agentName, path string) {
	ctx := context.Background()

	agentCfg, ok := app.cfg.Agents[agentName]
	if !ok {
		log.Printf("Agent %s not found", agentName)
		app.feishu.SendPrivateMessage(ctx, openID, fmt.Sprintf("Error: Agent %s not found", agentName), "text")
		return
	}

	agent, err := acp.New(agentCfg.Cmd, app.feishu, app.permissionMgr)
	if err != nil {
		log.Printf("Failed to start ACP: %v", err)
		app.feishu.SendPrivateMessage(ctx, openID, fmt.Sprintf("Error: Failed to start agent: %v", err), "text")
		return
	}

	if err := agent.Initialize(ctx); err != nil {
		agent.Close()
		log.Printf("Failed to initialize ACP: %v", err)
		app.feishu.SendPrivateMessage(ctx, openID, fmt.Sprintf("Error: Failed to initialize agent: %v", err), "text")
		return
	}

	sessionID, err := agent.CreateSession(ctx, path)
	if err != nil {
		agent.Close()
		log.Printf("Failed to create ACP session: %v", err)
		app.feishu.SendPrivateMessage(ctx, openID, fmt.Sprintf("Error: Failed to create session: %v", err), "text")
		return
	}

	groupChatID, err := app.feishu.CreateGroup(ctx, fmt.Sprintf("%s: %s", agentName, filepath.Base(path)), openID)
	if err != nil {
		agent.Close()
		log.Printf("Failed to create group: %v", err)
		app.feishu.SendPrivateMessage(ctx, openID, fmt.Sprintf("Error: Failed to create group: %v", err), "text")
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
	}
	if err := app.store.Set(groupChatID, &sessionInfo); err != nil {
		log.Printf("Warning: failed to save session: %v", err)
	}
	agent.SetSessionChatID(&sessionInfo)
	link, err := app.feishu.GetGroupShareLink(ctx, groupChatID)
	app.feishu.SendInteractiveCardToUser(ctx, openID, feishu.NewSessionFinishCard(agentName, path, link))
	welcomeMsg := fmt.Sprintf("会话创建成功\nAgent: %s\nPath: %s", agentName, path)
	if err := app.feishu.SendMessage(ctx, groupChatID, welcomeMsg); err != nil {
		log.Printf("Warning: failed to send welcome message: %v", err)
	}

	log.Printf("Session created: openID=%s, agent=%s, path=%s, groupChat=%s, acpSession=%s", openID, agentName, path, groupChatID, sessionID)
}

// handleMessage handles messages from Feishu groups
func (app *App) handleMessage(ctx context.Context, chatID, openID, content string) error {
	log.Printf("Message from %s in %s: %s", openID, chatID, content)

	info, ok := app.store.Get(chatID)
	if !ok {
		log.Printf("No session found for chat: %s", chatID)
		return nil
	}

	agent, ok := app.agents[chatID]
	if !ok {
		log.Printf("No ACP client found for chat: %s, attempting to restore...", chatID)
		agent, ok = app.restoreAgent(ctx, info)
		if !ok {
			app.feishu.SendMessage(ctx, chatID, "会话已过期或无法恢复")
			return nil
		}
	}

	if err := agent.SendMessage(ctx, info.ACPSessionID, content); err != nil {
		log.Printf("Failed to send message to ACP: %v", err)
		return err
	}
	agent.ResetStreaming(info)

	return nil
}

// restoreAgent restores an ACP client connection for a chat
func (app *App) restoreAgent(ctx context.Context, info *session.SessionInfo) (*acp.Client, bool) {
	agentConfig, ok := app.cfg.Agents[info.AgentName]
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

	if err := newAgent.LoadSession(ctx, info.ACPSessionID, info.Path); err != nil {
		newAgent.Close()
		log.Printf("Failed to load session: %v", err)
		return nil, false
	}

	newAgent.SetSessionChatID(info)
	app.agents[info.FeishuChatID] = newAgent
	log.Printf("Agent connection restored for chat: %s", info.FeishuChatID)
	return newAgent, true
}
