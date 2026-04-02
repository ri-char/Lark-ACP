package session

import (
	"context"
	"sync"

	"github.com/coder/acp-go-sdk"
	"github.com/ri-char/lark-acp/feishu"
	"github.com/ri-char/lark-acp/feishu/components"
	"github.com/ri-char/lark-acp/logger"
)

type Session struct {
	Mu           sync.Mutex `json:"-"`
	FeishuChatID string     `json:"feishu_chat_id"`
	ACPSessionID string     `json:"acp_session_id"`
	AgentName    string     `json:"agent_name"`
	Path         string     `json:"path"`

	Title *string `json:"title,omitempty"`

	toolCallIdToInfo   map[string]*components.ToolCallCard
	toolCallIdToInfoMu sync.Mutex

	PlanMsgId   *string    `json:"plan_msg_id,omitempty"`
	planMsgIdMu sync.Mutex `json:"-"`

	PinCardMsgId *string    `json:"pin_card_msg_id,omitempty"`
	pinCardMsgMu sync.Mutex `json:"-"`

	UsageMsgId *string `json:"usage_msg_id,omitempty"`
	UsageUsed  int     `json:"usage_used,omitempty"`
	UsageSize  int     `json:"usage_size,omitempty"`
	usageMsgMu sync.Mutex

	ModelId string            `json:"last_model_id,omitempty"`
	ModeId  string            `json:"last_mode_id,omitempty"`
	Models  []acp.ModelInfo   `json:"-"`
	Modes   []acp.SessionMode `json:"-"`
	infoMu  sync.RWMutex      `json:"-"`

	streamCard   *components.StreamCard `json:"-"`
	streamCardMu sync.Mutex             `json:"-"`
}

func (s *Session) UpdateInformationCardToFeishu(ctx context.Context) {
	s.infoMu.RLock()
	s.pinCardMsgMu.Lock()
	cardContent := feishu.GroupPinHeaderCard(s.AgentName, s.Path, s.Models, s.Modes, s.ModelId, s.ModeId, s.Title)
	feishu.SendOrUpdatePinCard(ctx, cardContent, s.FeishuChatID, &s.PinCardMsgId)
	s.pinCardMsgMu.Unlock()
	s.infoMu.RUnlock()
}

func (s *Session) UpdateUsageToFeishu(ctx context.Context, used, size int) {
	s.usageMsgMu.Lock()
	oldUsed := s.UsageUsed
	oldSize := s.UsageSize
	s.UsageUsed = used
	s.UsageSize = size
	if oldUsed != used || oldSize != size {
		cardContent := feishu.UsageHeaderCard(s.UsageUsed, s.UsageSize)
		feishu.SendOrUpdateTopNoticeCard(ctx, cardContent, s.FeishuChatID, &s.UsageMsgId)
	}
	s.usageMsgMu.Unlock()
}

func (s *Session) UpdatePlanToFeishu(ctx context.Context, plan []acp.PlanEntry) {
	s.planMsgIdMu.Lock()
	defer s.planMsgIdMu.Unlock()

	card := feishu.PlanCard(plan)
	msgIdPtr, err := feishu.SendOrUpdateInteractiveCard(context.Background(), s.FeishuChatID, card, s.PlanMsgId)
	if err != nil {
		logger.Debugf("Failed to send plan card to Feishu: %v", err)
	}
	if s.PlanMsgId == nil && msgIdPtr != nil {
		feishu.PinMessage(ctx, *msgIdPtr)
	}
	if msgIdPtr != nil {
		s.PlanMsgId = msgIdPtr
	}
}

func (s *Session) CloseStreamCard() {
	s.streamCardMu.Lock()
	defer s.streamCardMu.Unlock()

	if s.streamCard != nil {
		s.streamCard.Close()
		s.streamCard = nil
	}
}

func (s *Session) AddStreamingChunk(kind string, text string) {
	s.streamCardMu.Lock()
	defer s.streamCardMu.Unlock()

	if s.streamCard == nil {
		s.streamCard = components.NewStreamableCard(context.Background(), s.FeishuChatID, kind)
	} else if s.streamCard.CardType != kind {
		go s.streamCard.Close()
		s.streamCard = components.NewStreamableCard(context.Background(), s.FeishuChatID, kind)
	}
	s.streamCard.WriteChunk(text)
}

func (s *Session) SetMode(modeId string) {
	s.infoMu.Lock()
	s.ModeId = modeId
	s.infoMu.Unlock()
	SessionStoreInstance.Save()
}
func (s *Session) SetModel(modelId string) {
	s.infoMu.Lock()
	s.ModelId = modelId
	s.infoMu.Unlock()
	SessionStoreInstance.Save()
}

func (s *Session) GetMode() string {
	s.infoMu.RLock()
	defer s.infoMu.RUnlock()
	return s.ModeId
}
func (s *Session) GetModel() string {
	s.infoMu.RLock()
	defer s.infoMu.RUnlock()
	return s.ModelId
}

func (s *Session) SetModes(modes []acp.SessionMode) {
	s.infoMu.Lock()
	s.Modes = modes
	s.infoMu.Unlock()
}

func (s *Session) SetModels(models []acp.ModelInfo) {
	s.infoMu.Lock()
	s.Models = models
	s.infoMu.Unlock()
}

func (sessionInfo *Session) GetOrInitToolcall(toolCallId string) *components.ToolCallCard {
	sessionInfo.toolCallIdToInfoMu.Lock()
	defer sessionInfo.toolCallIdToInfoMu.Unlock()
	if sessionInfo.toolCallIdToInfo == nil {
		sessionInfo.toolCallIdToInfo = make(map[string]*components.ToolCallCard)
	}
	toolCallInfo, ok := sessionInfo.toolCallIdToInfo[toolCallId]
	if !ok {
		toolCallInfo = components.NewToolCallCard()
		sessionInfo.toolCallIdToInfo[toolCallId] = toolCallInfo
	}
	return toolCallInfo
}
func (s *Session) GetTitle() *string {
	s.infoMu.RLock()
	defer s.infoMu.RUnlock()
	return s.Title
}
func (s *Session) SetTitle(title *string) {
	s.infoMu.Lock()
	defer s.infoMu.Unlock()
	s.Title = title
}