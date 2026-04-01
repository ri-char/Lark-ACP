package session

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/ri-char/lark-acp/feishu"
	"github.com/ri-char/lark-acp/feishu/components"
)

// PermissionResponse represents user's response to a permission request
type PermissionResponse struct {
	OptionId  acpsdk.PermissionOptionId
	Cancelled bool
}

// PendingPermission represents a waiting permission request
type PendingPermission struct {
	SessionID string
	Options   []acpsdk.PermissionOption
	Response  chan PermissionResponse
	ToolCall  acpsdk.ToolCallUpdate
}

// PermissionManager manages pending permission requests
type PermissionManager struct {
	mu      sync.RWMutex
	pending map[string]*PendingPermission // requestID -> pending permission
}

// NewPermissionManager creates a new permission manager
func NewPermissionManager() *PermissionManager {
	return &PermissionManager{
		pending: make(map[string]*PendingPermission),
	}
}

// Add adds a pending permission request and returns the response channel
func (pm *PermissionManager) Add(requestID string, p *PendingPermission) {
	pm.mu.Lock()
	pm.pending[requestID] = p
	pm.mu.Unlock()
}

// Get gets a pending permission request
func (pm *PermissionManager) Get(requestID string) (*PendingPermission, bool) {
	pm.mu.RLock()
	p, ok := pm.pending[requestID]
	pm.mu.RUnlock()
	return p, ok
}

// Remove removes a pending permission request
func (pm *PermissionManager) Remove(requestID string) {
	pm.mu.Lock()
	delete(pm.pending, requestID)
	pm.mu.Unlock()
}

type SessionInfo struct {
	Mu               sync.Mutex `json:"-"`
	FeishuChatID     string     `json:"feishu_chat_id"`
	ACPSessionID     string     `json:"acp_session_id"`
	AgentName        string     `json:"agent_name"`
	Path             string     `json:"path"`
	ToolCallIdToInfo map[string]*components.ToolCallCard
	PlanMsgId        *string `json:"plan_msg_id,omitempty"`
	PinCardMsgId     *string `json:"pin_card_msg_id,omitempty"`
	UsageMsgId       *string `json:"usage_msg_id,omitempty"`
	Title            *string `json:"title,omitempty"`

	LastModelId string `json:"last_model_id,omitempty"`
	LastModeId  string `json:"last_mode_id,omitempty"`
	Models      *acpsdk.SessionModelState
	Modes       *acpsdk.SessionModeState
	UsageUsed   int `json:"usage_used,omitempty"`
	UsageSize   int `json:"usage_size,omitempty"`

	StreamCard *components.StreamCard
}

func (sessionInfo *SessionInfo) UpdateInformationCard(ctx context.Context, client *feishu.Client) {
	cardContent := feishu.GroupPinHeaderCard(sessionInfo.AgentName, sessionInfo.Path, sessionInfo.Models, sessionInfo.Modes, sessionInfo.Title)
	client.SendOrUpdatePinCard(ctx, cardContent, sessionInfo.FeishuChatID, &sessionInfo.PinCardMsgId)
}
func (sessionInfo *SessionInfo) UpdateUsage(ctx context.Context, client *feishu.Client) {
	cardContent := feishu.UsageHeaderCard(sessionInfo.UsageUsed, sessionInfo.UsageSize)
	client.SendOrUpdateTopNoticeCard(ctx, cardContent, sessionInfo.FeishuChatID, &sessionInfo.UsageMsgId)
}

type SessionStore struct {
	mu       sync.RWMutex
	Sessions map[string]*SessionInfo `json:"sessions"` // feishu_chat_id -> session info
	filePath string
}

func NewStore() (*SessionStore, error) {
	filePath := getSessionPath()
	store := &SessionStore{
		Sessions: make(map[string]*SessionInfo),
		filePath: filePath,
	}

	// Load existing sessions if file exists
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return nil, err
	}

	if len(data) > 0 {
		if err := json.Unmarshal(data, store); err != nil {
			return nil, err
		}
	}
	for _, info := range store.Sessions {
		if info.ToolCallIdToInfo == nil {
			info.ToolCallIdToInfo = make(map[string]*components.ToolCallCard)
		}
	}

	return store, nil
}

func (s *SessionStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveUnlocked()
}

func (s *SessionStore) saveUnlocked() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

func (s *SessionStore) Set(chatID string, info *SessionInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Sessions[chatID] = info
	return s.saveUnlocked()
}

func (s *SessionStore) Get(chatID string) (*SessionInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	info, ok := s.Sessions[chatID]
	return info, ok
}

func (s *SessionStore) GetByACPSession(agentName, acpSessionID string) (*SessionInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, info := range s.Sessions {
		if info.ACPSessionID == acpSessionID && info.AgentName == agentName {
			return info, true
		}
	}
	return nil, false
}

func (s *SessionStore) Delete(chatID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Sessions, chatID)
	return s.saveUnlocked()
}

func getSessionPath() string {
	path, err := os.UserConfigDir()
	if err != nil {
		return "session.json"
	}
	return filepath.Join(path, "lark-acp", "session.json")
}
