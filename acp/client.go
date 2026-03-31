package acp

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/coder/acp-go-sdk"
	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/ri-char/lark-acp/feishu"
	"github.com/ri-char/lark-acp/session"
)

// Client implements the acp.Client interface and handles ACP communication
type Client struct {
	cmd           *exec.Cmd
	conn          *acpsdk.ClientSideConnection
	mu            sync.Mutex
	sessions      map[string]*session.SessionInfo // sessionID -> chatID mapping for callbacks
	feishu        *feishu.Client
	permissionMgr *session.PermissionManager
	terminals     *TerminalManager
	Capabilities  []string
}

// New creates a new ACP client by launching the agent command
func New(cmdStr []string, feishu *feishu.Client, permissionMgr *session.PermissionManager) (*Client, error) {
	cmd := exec.Command(cmdStr[0], cmdStr[1:]...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	c := &Client{
		cmd:           cmd,
		sessions:      make(map[string]*session.SessionInfo),
		feishu:        feishu,
		permissionMgr: permissionMgr,
		terminals:     NewTerminalManager(),
	}

	// Create client-side connection with our handler
	wrappedStdout := NewJSONRPCReader(stdout)
	c.conn = acpsdk.NewClientSideConnection(c, stdin, wrappedStdout)

	return c, nil
}

// contains checks if a string exists in a slice of strings
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Initialize initializes the ACP connection
func (c *Client) Initialize(ctx context.Context) error {
	resp, err := c.conn.Initialize(ctx, acpsdk.InitializeRequest{
		ProtocolVersion: acpsdk.ProtocolVersionNumber,
		ClientCapabilities: acpsdk.ClientCapabilities{
			Fs: acpsdk.FileSystemCapability{
				ReadTextFile:  true,
				WriteTextFile: true,
			},
			Terminal: true,
		},
	})
	if resp.AgentCapabilities.LoadSession {
		c.Capabilities = append(c.Capabilities, "load_session")
	}
	if resp.AgentCapabilities.SessionCapabilities.List != nil {
		c.Capabilities = append(c.Capabilities, "list_session")
	}
	return err
}

// CreateSession creates a new ACP session with the given working directory
// cwd must be an absolute path
func (c *Client) CreateSession(ctx context.Context, cwd string) (string, *acpsdk.SessionModelState, *acpsdk.SessionModeState, error) {
	resp, err := c.conn.NewSession(ctx, acpsdk.NewSessionRequest{
		Cwd:        cwd,
		McpServers: []acpsdk.McpServer{},
	})
	if err != nil {
		return "", nil, nil, err
	}
	return string(resp.SessionId), resp.Models, resp.Modes, nil
}

// LoadSession loads an existing ACP session
// cwd must be an absolute path
func (c *Client) LoadSession(ctx context.Context, sessionID, cwd string) (*acpsdk.SessionModelState, *acpsdk.SessionModeState, error) {
	if !contains(c.Capabilities, "load_session") {
		return nil, nil, fmt.Errorf("agent does not support load_session capability")
	}
	resp, err := c.conn.LoadSession(ctx, acpsdk.LoadSessionRequest{
		SessionId:  acpsdk.SessionId(sessionID),
		Cwd:        cwd,
		McpServers: []acpsdk.McpServer{},
	})

	return resp.Models, resp.Modes, err
}

func (c *Client) SetModel(ctx context.Context, sessionID, modelId string) error {
	_, err := c.conn.UnstableSetSessionModel(ctx, acpsdk.UnstableSetSessionModelRequest{
		SessionId: acpsdk.SessionId(sessionID),
		ModelId:   acpsdk.UnstableModelId(modelId),
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) SetMode(ctx context.Context, sessionID, modeId string) error {
	_, err := c.conn.SetSessionMode(ctx, acpsdk.SetSessionModeRequest{
		SessionId: acpsdk.SessionId(sessionID),
		ModeId:    acpsdk.SessionModeId(modeId),
	})
	if err != nil {
		return err
	}
	return nil
}

// SendMessage sends a prompt to the ACP session
func (c *Client) SendMessage(ctx context.Context, sessionID, content string) error {
	_, err := c.conn.Prompt(ctx, acpsdk.PromptRequest{
		SessionId: acpsdk.SessionId(sessionID),
		Prompt:    []acpsdk.ContentBlock{acpsdk.TextBlock(content)},
	})
	return err
}

func (c *Client) ListSessions(ctx context.Context) ([]acp.UnstableSessionInfo, error) {
	if !contains(c.Capabilities, "list_session") {
		return nil, fmt.Errorf("agent does not support list_session capability")
	}
	var cursor *string
	var sessionIDs []acp.UnstableSessionInfo
	for {
		resp, err := c.conn.UnstableListSessions(ctx, acpsdk.UnstableListSessionsRequest{
			Cursor: cursor,
		})
		if err != nil {
			return nil, err
		}
		sessionIDs = append(sessionIDs, resp.Sessions...)
		if resp.NextCursor == nil {
			break
		}
		cursor = resp.NextCursor
	}
	return sessionIDs, nil
}

// SetSessionChatID associates a session with a Feishu chat ID for callbacks
func (c *Client) SetSessionChatID(session *session.SessionInfo) {
	c.mu.Lock()
	c.sessions[session.ACPSessionID] = session
	c.mu.Unlock()
}

// Close closes the ACP connection and stops the agent process
func (c *Client) Close() error {
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		return c.cmd.Wait()
	}
	return nil
}

// GetConnection returns the underlying connection for direct use
func (c *Client) GetConnection() *acpsdk.ClientSideConnection {
	return c.conn
}

// --- Implement acpsdk.Client interface ---

// ReadTextFile handles file read requests from the agent
func (c *Client) ReadTextFile(ctx context.Context, p acpsdk.ReadTextFileRequest) (acpsdk.ReadTextFileResponse, error) {
	if !filepath.IsAbs(p.Path) {
		return acpsdk.ReadTextFileResponse{}, fmt.Errorf("path must be absolute: %s", p.Path)
	}
	b, err := os.ReadFile(p.Path)
	if err != nil {
		return acpsdk.ReadTextFileResponse{}, err
	}
	content := string(b)
	if p.Line != nil || p.Limit != nil {
		lines := strings.Split(content, "\n")
		start := 0
		if p.Line != nil && *p.Line > 0 {
			if *p.Line-1 > 0 {
				start = *p.Line - 1
			}
			if start > len(lines) {
				start = len(lines)
			}
		}
		end := len(lines)
		if p.Limit != nil && *p.Limit > 0 && start+*p.Limit < end {
			end = start + *p.Limit
		}
		content = strings.Join(lines[start:end], "\n")
	}
	return acpsdk.ReadTextFileResponse{Content: content}, nil
}

// WriteTextFile handles file write requests from the agent
func (c *Client) WriteTextFile(ctx context.Context, p acpsdk.WriteTextFileRequest) (acpsdk.WriteTextFileResponse, error) {
	if !filepath.IsAbs(p.Path) {
		return acpsdk.WriteTextFileResponse{}, fmt.Errorf("path must be absolute: %s", p.Path)
	}
	if dir := filepath.Dir(p.Path); dir != "" {
		_ = os.MkdirAll(dir, 0755)
	}
	return acpsdk.WriteTextFileResponse{}, os.WriteFile(p.Path, []byte(p.Content), 0644)
}

// RequestPermission handles permission requests from the agent
func (c *Client) RequestPermission(ctx context.Context, p acpsdk.RequestPermissionRequest) (acpsdk.RequestPermissionResponse, error) {
	if len(p.Options) == 0 {
		return acpsdk.RequestPermissionResponse{
			Outcome: acpsdk.RequestPermissionOutcome{
				Cancelled: &acpsdk.RequestPermissionOutcomeCancelled{},
			},
		}, nil
	}

	// 获取 session info
	sessionInfo := c.sessions[string(p.SessionId)]
	if sessionInfo == nil {
		return acpsdk.RequestPermissionResponse{
			Outcome: acpsdk.RequestPermissionOutcome{
				Cancelled: &acpsdk.RequestPermissionOutcomeCancelled{},
			},
		}, nil
	}

	// 生成 request ID
	requestID := fmt.Sprintf("perm_%s", p.ToolCall.ToolCallId)

	// 发送权限卡片
	card := feishu.PermissionCard(string(p.SessionId), requestID, p.Options, p.ToolCall)
	if _, err := c.feishu.SendInteractiveCard(ctx, sessionInfo.FeishuChatID, card); err != nil {
		log.Printf("Failed to send permission card: %v", err)
		return acpsdk.RequestPermissionResponse{
			Outcome: acpsdk.RequestPermissionOutcome{
				Cancelled: &acpsdk.RequestPermissionOutcomeCancelled{},
			},
		}, nil
	}

	// 等待用户响应
	pending := &session.PendingPermission{
		SessionID: string(p.SessionId),
		Options:   p.Options,
		ToolCall:  p.ToolCall,
		Response:  make(chan session.PermissionResponse, 1),
	}
	c.permissionMgr.Add(requestID, pending)
	defer c.permissionMgr.Remove(requestID)

	select {
	case resp := <-pending.Response:
		if resp.Cancelled {
			return acpsdk.RequestPermissionResponse{
				Outcome: acpsdk.RequestPermissionOutcome{
					Cancelled: &acpsdk.RequestPermissionOutcomeCancelled{},
				},
			}, nil
		}
		return acpsdk.RequestPermissionResponse{
			Outcome: acpsdk.RequestPermissionOutcome{
				Selected: &acpsdk.RequestPermissionOutcomeSelected{OptionId: resp.OptionId},
			},
		}, nil
	case <-ctx.Done():
		return acpsdk.RequestPermissionResponse{
			Outcome: acpsdk.RequestPermissionOutcome{
				Cancelled: &acpsdk.RequestPermissionOutcomeCancelled{},
			},
		}, nil
	}
}

func (c *Client) updateToolCall(ToolCall *acpsdk.SessionUpdateToolCall, ToolCallUpdate *acpsdk.SessionToolCallUpdate, sessionInfo *session.SessionInfo) {
	var toolCallId string
	if ToolCall != nil {
		toolCallId = string(ToolCall.ToolCallId)
	} else if ToolCallUpdate != nil {
		toolCallId = string(ToolCallUpdate.ToolCallId)
	}
	toolCallInfo, ok := sessionInfo.ToolCallIdToInfo[toolCallId]
	if !ok {
		toolCallInfo = &session.ToolCallIdInfo{}
		sessionInfo.ToolCallIdToInfo[string(ToolCall.ToolCallId)] = toolCallInfo
	}
	if ToolCall != nil {
		toolCallInfo.Content = ToolCall.Content
		toolCallInfo.Locations = ToolCall.Locations
		toolCallInfo.Title = ToolCall.Title
		toolCallInfo.Status = ToolCall.Status
		toolCallInfo.Kind = ToolCall.Kind
	}
	if ToolCallUpdate != nil {
		if len(ToolCallUpdate.Content) > 0 {
			toolCallInfo.Content = ToolCallUpdate.Content
		}
		if ToolCallUpdate.Locations != nil {
			toolCallInfo.Locations = ToolCallUpdate.Locations
		}
		if ToolCallUpdate.Title != nil {
			toolCallInfo.Title = *ToolCallUpdate.Title
		}
		if ToolCallUpdate.Status != nil {
			toolCallInfo.Status = *ToolCallUpdate.Status
		}
		if ToolCallUpdate.Kind != nil {
			toolCallInfo.Kind = *ToolCallUpdate.Kind
		}
	}
	card := feishu.ToolCallCard(toolCallInfo)
	msgIdPtr, err := c.feishu.SendOrUpdateInteractiveCard(context.Background(), sessionInfo.FeishuChatID, card, toolCallInfo.MsgId)
	if err != nil {
		log.Printf("Failed to send tool call card to Feishu: %v", err)
	}
	if msgIdPtr != nil {
		toolCallInfo.MsgId = msgIdPtr
	}
}

// SessionUpdate handles session update notifications from the agent
func (c *Client) SessionUpdate(ctx context.Context, n acpsdk.SessionNotification) error {
	u := n.Update
	sessionInfo, ok := c.sessions[string(n.SessionId)]
	if !ok {
		log.Printf("Received session update for unknown session: %s", n.SessionId)
		return nil
	}

	sessionInfo.Mu.Lock()
	defer sessionInfo.Mu.Unlock()

	switch {
	case u.AgentMessageChunk != nil:
		if u.AgentMessageChunk.Content.Text != nil {
			content := u.AgentMessageChunk.Content.Text.Text
			c.AddStreamingChunk(sessionInfo, "message", content)
		}
	case u.ToolCall != nil || u.ToolCallUpdate != nil:
		c.ResetStreaming(sessionInfo)
		c.updateToolCall(u.ToolCall, u.ToolCallUpdate, sessionInfo)
	case u.AgentThoughtChunk != nil:
		if u.AgentThoughtChunk.Content.Text != nil {
			content := u.AgentThoughtChunk.Content.Text.Text
			c.AddStreamingChunk(sessionInfo, "thought", content)
		}
	case u.Plan != nil:
		c.ResetStreaming(sessionInfo)
		card := feishu.PlanCard(u.Plan.Entries)
		msgIdPtr, err := c.feishu.SendOrUpdateInteractiveCard(context.Background(), sessionInfo.FeishuChatID, card, sessionInfo.PlanMsgId)
		if err != nil {
			log.Printf("Failed to send plan card to Feishu: %v", err)
		}
		if sessionInfo.PlanMsgId == nil && msgIdPtr != nil {
			c.feishu.PutTopNotice(ctx, sessionInfo.FeishuChatID, *msgIdPtr)
		}
		if msgIdPtr != nil {
			sessionInfo.PlanMsgId = msgIdPtr
		}
	case u.UserMessageChunk != nil:
		// Skip user message chunks, we already know what user sent
		return nil
	case u.CurrentModeUpdate != nil:
		sessionInfo.LastModeId = string(u.CurrentModeUpdate.CurrentModeId)
		if sessionInfo.Modes != nil {
			sessionInfo.Modes.CurrentModeId = acpsdk.SessionModeId(u.CurrentModeUpdate.CurrentModeId)
		}
		c.feishu.SendOrUpdatePinCard(ctx, sessionInfo)
	}

	return nil
}

func (c *Client) ResetStreaming(s *session.SessionInfo) {
	if s.InStreaming {
		s.StreamingId += 1
		c.feishu.UpdateCard(context.Background(), s.StreamingCardId, feishu.StreamingCardEndSetting(), s.StreamingId)
	}
	s.InStreaming = false
	s.StreamingText = ""
	s.StreamingType = ""
	s.StreamingCardId = ""
	s.StreamingId = 0
}

func (c *Client) AddStreamingChunk(s *session.SessionInfo, kind string, text string) {
	if s.InStreaming && s.StreamingType != kind {
		c.ResetStreaming(s)
	}

	if !s.InStreaming {
		log.Printf("Create new chunk: %s", text)
		s.StreamingText += text
		s.StreamingType = kind
		card_id, err := c.feishu.CreateCard(context.Background(), feishu.StreamingCard(kind, s.StreamingText))
		if err != nil {
			log.Printf("Failed to create streaming card: %v", err)
			return
		}
		_, err = c.feishu.SendInteractiveCardById(context.Background(), s.FeishuChatID, card_id)
		if err != nil {
			log.Printf("Failed to send streaming card: %v", err)
			return
		}
		s.InStreaming = true
		s.StreamingCardId = card_id
	} else {
		log.Printf("Add streaming chunk: %s", text)
		s.StreamingText += text
		s.StreamingId += 1
		c.feishu.UpdateCardElement(context.Background(), s.StreamingCardId, "markdown_main", s.StreamingText, s.StreamingId)
	}

}

// Done returns a channel that closes when the connection is closed
func (c *Client) Done() <-chan struct{} {
	return c.conn.Done()
}

// Ensure Client implements acpsdk.Client interface
var _ acpsdk.Client = (*Client)(nil)

// Ensure Client implements io.Closer interface
var _ io.Closer = (*Client)(nil)
