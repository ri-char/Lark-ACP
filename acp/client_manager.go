package acp

import (
	"sync"

	"github.com/ri-char/lark-acp/config"
	"github.com/ri-char/lark-acp/logger"
)

type ACPClientManager struct {
	agents      map[string]*Client // chatId -> Client
	agentConfig []config.AgentConfig
	mu          sync.RWMutex
}

var (
	ACPClientManagerInstance ACPClientManager
)

func InitACPClientManager(agentConfig []config.AgentConfig) {
	ACPClientManagerInstance = ACPClientManager{
		agents:      make(map[string]*Client),
		agentConfig: agentConfig,
	}
}

func (m *ACPClientManager) CloseAll() {
	m.mu.Lock()
	for _, agent := range m.agents {
		agent.Close()
	}
	m.mu.Unlock()
}

func (m *ACPClientManager) Get(chatID string) (*Client, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	agent, ok := m.agents[chatID]
	return agent, ok
}

func (m *ACPClientManager) Set(chatID string, agent *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agents[chatID] = agent
}

func (m *ACPClientManager) Delete(chatID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.agents, chatID)
}

func (m *ACPClientManager) CloseAgent(chatID string, acpSessionID string) {
	m.mu.Lock()
	agent, ok := m.agents[chatID]
	if !ok {
		m.mu.Unlock()
		return
	}

	delete(m.agents, chatID)
	m.mu.Unlock()

	if m.IsAgentInUse(agent) {
		logger.Debugf("Agent for session %s is still in use by another chat, not closing", acpSessionID)
	} else {
		logger.Debugf("Closing agent for session %s", acpSessionID)
		agent.Close()
	}
}

func (m *ACPClientManager) IsAgentInUse(agent *Client) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, a := range m.agents {
		if a == agent {
			return true
		}
	}
	return false
}

func (m *ACPClientManager) GetAllAgentNames() []string {
	agentNames := make([]string, 0, len(m.agentConfig))
	for _, agents := range m.agentConfig {
		agentNames = append(agentNames, agents.Id)
	}
	return agentNames
}

func (m *ACPClientManager) FindAgentConfigById(id string) (agentCfg *config.AgentConfig, exist bool) {
	for _, agent := range m.agentConfig {
		if agent.Id == id {
			agentCfg = &agent
			exist = true
			break
		}
	}
	return
}
