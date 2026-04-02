package acp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ri-char/lark-acp/logger"

	"github.com/coder/acp-go-sdk"
	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/ri-char/lark-acp/config"
	"github.com/ri-char/lark-acp/feishu"
	"github.com/ri-char/lark-acp/session"
)

// Client implements the acp.Client interface and handles ACP communication
type Client struct {
	cmd          *exec.Cmd
	conn         *acpsdk.ClientSideConnection
	mu           sync.Mutex
	sessions     map[string]*session.Session // sessionID -> chatID mapping for callbacks
	terminals    *TerminalManager
	Capabilities []string
	closed       bool
}

// New creates a new ACP client by launching the agent command
func New(config *config.AgentConfig, maps *map[string]*Client) (*Client, error) {
	cmd := exec.Command(config.Cmd[0], config.Cmd[1:]...)

	cmd.Env = os.Environ()
	for k, v := range config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	c := &Client{
		cmd:       cmd,
		sessions:  make(map[string]*session.Session),
		terminals: NewTerminalManager(),
	}

	c.conn = acpsdk.NewClientSideConnection(c, stdin, stdout)
	c.conn.SetLogger(slog.Default().With("lib", "acp"))
	go c.NoticeWhenDone(maps)
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
func (c *Client) SetSessionChatID(session *session.Session) {
	c.mu.Lock()
	c.sessions[session.ACPSessionID] = session
	c.mu.Unlock()
}

// Close closes the ACP connection and stops the agent process
func (c *Client) Close() error {
	c.closed = true
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
	logger.Debug("RequestPermission from acp")
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

	toolCallId := string(p.ToolCall.ToolCallId)
	toolCallInfo := sessionInfo.GetOrInitToolcall(toolCallId)
	toolCallInfo.UpdateByToolCallUpdate(&p.ToolCall)
	toolCallInfo.SetPermissionList(p.Options)
	requestID := session.GetPermissionManager().GetRequestID()
	toolCallInfo.SetPermissionRequestID(requestID)

	// 等待用户响应
	pending := &session.PendingPermission{
		ToolCard: toolCallInfo,
		Response: make(chan session.PermissionResponse, 1),
	}
	session.GetPermissionManager().Add(requestID, pending)
	defer session.GetPermissionManager().Remove(requestID)

	err := toolCallInfo.UpdateFeishu(ctx, sessionInfo.FeishuChatID)

	if err != nil {
		logger.Debugf("Failed to send permission card: %v", err)
		return acpsdk.RequestPermissionResponse{
			Outcome: acpsdk.RequestPermissionOutcome{
				Cancelled: &acpsdk.RequestPermissionOutcomeCancelled{},
			},
		}, nil
	}

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

func (c *Client) updateToolCall(ToolCall *acpsdk.SessionUpdateToolCall, ToolCallUpdate *acpsdk.SessionToolCallUpdate, sessionInfo *session.Session) {
	var toolCallId string
	if ToolCall != nil {
		toolCallId = string(ToolCall.ToolCallId)
	} else if ToolCallUpdate != nil {
		toolCallId = string(ToolCallUpdate.ToolCallId)
	}
	toolCallInfo := sessionInfo.GetOrInitToolcall(toolCallId)
	if ToolCall != nil {
		toolCallInfo.UpdateBySessionUpdateToolCall(ToolCall)
	}
	if ToolCallUpdate != nil {
		toolCallInfo.UpdateBySessionToolCallUpdate(ToolCallUpdate)
	}
	toolCallInfo.UpdateFeishu(context.Background(), sessionInfo.FeishuChatID)
}

// SessionUpdate handles session update notifications from the agent
func (c *Client) SessionUpdate(ctx context.Context, n acpsdk.SessionNotification) error {
	u := n.Update
	session, ok := c.sessions[string(n.SessionId)]
	if !ok {
		logger.Debugf("Received session update for unknown session: %s", n.SessionId)
		return nil
	}

	session.Mu.Lock()
	defer session.Mu.Unlock()

	switch {
	case u.AgentMessageChunk != nil:
		// logger.Debug("SessionUpdate from acp", "type", "AgentMessageChunk")
		if u.AgentMessageChunk.Content.Text != nil {
			content := u.AgentMessageChunk.Content.Text.Text
			session.AddStreamingChunk("message", content)
		}
	case u.ToolCall != nil || u.ToolCallUpdate != nil:
		logger.Debug("SessionUpdate from acp", "type", "ToolCall/ToolCallUpdate")
		session.CloseStreamCard()
		c.updateToolCall(u.ToolCall, u.ToolCallUpdate, session)
	case u.AgentThoughtChunk != nil:
		// logger.Debug("SessionUpdate from acp", "type", "AgentThoughtChunk")
		if u.AgentThoughtChunk.Content.Text != nil {
			content := u.AgentThoughtChunk.Content.Text.Text
			session.AddStreamingChunk("thought", content)
		}
	case u.Plan != nil:
		logger.Debug("SessionUpdate from acp", "type", "Plan")
		session.CloseStreamCard()
		session.UpdatePlanToFeishu(ctx, u.Plan.Entries)
	case u.UserMessageChunk != nil:
		logger.Debug("SessionUpdate from acp", "type", "UserMessageChunk")
		// Skip user message chunks, we already know what user sent
	case u.CurrentModeUpdate != nil:
		logger.Debug("SessionUpdate from acp", "type", "CurrentModeUpdate")
		session.SetMode(string(u.CurrentModeUpdate.CurrentModeId))
		session.UpdateInformationCardToFeishu(ctx)
	case u.UsageUpdate != nil:
		logger.Debug("SessionUpdate from acp", "type", "UsageUpdate")
		session.UpdateUsageToFeishu(ctx, u.UsageUpdate.Used, u.UsageUpdate.Size)
	case u.SessionInfoUpdate != nil:
		logger.Debug("SessionUpdate from acp", "type", "SessionInfoUpdate")
		if u.SessionInfoUpdate.Title == nil {
			break
		}
		oldTitle := session.GetTitle()
		session.SetTitle(u.SessionInfoUpdate.Title)
		if oldTitle == nil || *oldTitle != *u.SessionInfoUpdate.Title {
			session.UpdateInformationCardToFeishu(ctx)
		}
	default:
		logger.Debug("SessionUpdate from acp", "type", "unknown")
	}
	return nil
}

func (c *Client) NoticeWhenDone(maps *map[string]*Client) {
	_, ok := <-c.conn.Done()
	c.mu.Lock()
	defer c.mu.Unlock()
	if !ok && !c.closed {
		for _, sessionInfo := range c.sessions {
			feishu.SendMessage(context.Background(), sessionInfo.FeishuChatID, "Agent异常退出")
		}

		for k, v := range *maps {
			if v == c {
				delete(*maps, k)
			}
		}
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
