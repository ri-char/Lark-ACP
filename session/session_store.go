package session

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type SessionStore struct {
	mu       sync.RWMutex
	Sessions map[string]*Session `json:"sessions"` // feishu_chat_id -> session info
	filePath string
}

var (
	SessionStoreInstance *SessionStore
)

func InitStore(ctx context.Context) error {
	filePath := getSessionPath()
	SessionStoreInstance = &SessionStore{
		Sessions: make(map[string]*Session),
		filePath: filePath,
	}
	SessionStoreInstance.mu.Lock()
	defer SessionStoreInstance.mu.Unlock()
	// Load existing sessions if file exists
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if len(data) > 0 {
		if err := json.Unmarshal(data, SessionStoreInstance); err != nil {
			return err
		}
	}

	return nil
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

func (s *SessionStore) Set(chatID string, info *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Sessions[chatID] = info
	return s.saveUnlocked()
}

func (s *SessionStore) Get(chatID string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	info, ok := s.Sessions[chatID]
	return info, ok
}

func (s *SessionStore) GetByACPSession(agentName, acpSessionID string) (*Session, bool) {
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
