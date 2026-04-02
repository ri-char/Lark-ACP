package session

import (
	"crypto/rand"
	"encoding/hex"
	"sync"

	 "github.com/coder/acp-go-sdk"
	"github.com/ri-char/lark-acp/feishu/components"
)

// PermissionResponse represents user's response to a permission request
type PermissionResponse struct {
	OptionId  acp.PermissionOptionId
	Cancelled bool
}

// PendingPermission represents a waiting permission request
type PendingPermission struct {
	ToolCard *components.ToolCallCard
	Response chan PermissionResponse
}

// PermissionManager manages pending permission requests
type PermissionManager struct {
	mu      sync.RWMutex
	pending map[string]*PendingPermission // requestID -> pending permission
}

var (
	instance *PermissionManager
	once     sync.Once
)

// NewPermissionManager creates a new permission manager
func GetPermissionManager() *PermissionManager {
	once.Do(func() {
		instance = &PermissionManager{
			pending: make(map[string]*PendingPermission),
		}
	})
	return instance
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

func (pm *PermissionManager) GetRequestID() string {
	var randReqIdBytes [8]byte
	rand.Read(randReqIdBytes[:])
	requestID := hex.EncodeToString(randReqIdBytes[:])
	return requestID
}
