package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	acpsdk "github.com/coder/acp-go-sdk"
)

// PermissionResponse represents user's response to a permission request
type PermissionResponse struct {
	OptionId acpsdk.PermissionOptionId
	Cancelled bool
}

// PendingPermission represents a waiting permission request
type PendingPermission struct {
	SessionID string
	Options   []acpsdk.PermissionOption
	Response  chan PermissionResponse
	ToolCall  acpsdk.RequestPermissionToolCall
}

// PermissionManager manages pending permission requests
type PermissionManager struct {
	mu          sync.RWMutex
	pending     map[string]*PendingPermission // requestID -> pending permission
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

type ToolCallIdInfo struct {
	Content []acpsdk.ToolCallContent `json:"content,omitempty"`
	Kind    acpsdk.ToolKind          `json:"kind,omitempty"`
	Locations []acpsdk.ToolCallLocation `json:"locations,omitempty"`
	Status    acpsdk.ToolCallStatus     `json:"status,omitempty"`
	Title     string                    `json:"title"`
	MsgId     *string                   `json:"msgId,omitempty"`
}

type SessionInfo struct {
	Mu                sync.Mutex `json:"-"`
	FeishuChatID      string `json:"feishu_chat_id"`
	ACPSessionID      string `json:"acp_session_id"`
	AgentName         string `json:"agent_name"`
	Path              string `json:"path"`
	ToolCallIdToInfo  map[string]*ToolCallIdInfo `json:"tool_call_map"`
	PlanMsgId         *string `json:"plan_msg_id,omitempty"`
	PinCardMsgId      *string `json:"pin_card_msg_id,omitempty"`

	LastModelId       string `json:"last_model_id,omitempty"`
	LastModeId        string `json:"last_mode_id,omitempty"`
	Models *acpsdk.SessionModelState
	Modes *acpsdk.SessionModeState

	InStreaming		  bool   `json:"in_streaming"`
	StreamingText     string `json:"streaming_text,omitempty"`
	StreamingId       int `json:"streaming_id,omitempty"`
	StreamingType    	string `json:"streaming_type,omitempty"`
	StreamingCardId    string `json:"streaming_card_id,omitempty"`
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
			info.ToolCallIdToInfo = make(map[string]*ToolCallIdInfo)
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

func (s *SessionStore) GetByACPSession(acpSessionID string) (*SessionInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, info := range s.Sessions {
		if info.ACPSessionID == acpSessionID {
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
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "lark-acp", "session.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "lark-acp", "session.json")
}