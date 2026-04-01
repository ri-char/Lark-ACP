package components

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/ri-char/lark-acp/feishu"
	"github.com/ri-char/lark-acp/logger"
)

type StreamCard struct {
	client   *feishu.Client
	chatId   string
	cardId   *string
	sequence int
	fullText string
	CardType string

	update chan bool  // signal to trigger API update
	mu     sync.Mutex // protect fullText and cardId
	wg     sync.WaitGroup
	ctx    context.Context
}

// NewStreamableCard creates a new StreamableCard without sending it immediately
func NewStreamableCard(ctx context.Context, client *feishu.Client, chatId string, cardType string) *StreamCard {
	c := &StreamCard{
		client:   client,
		chatId:   chatId,
		update:   make(chan bool, 1),
		CardType: cardType,
		ctx:      ctx,
	}
	c.wg.Add(1)
	go c.updateWorker(ctx)
	return c
}

// updateWorker handles API calls in a separate goroutine
func (s *StreamCard) updateWorker(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case v, ok := <-s.update:
			if v || !ok {
				return
			}
			s.doUpdate(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func streamingCard(ty string, text string) string {
	card := map[string]any{
		"schema": "2.0",

		"config": map[string]any{
			"streaming_mode": true,
		},
		"body": map[string]any{
			"elements": []map[string]any{
				{
					"tag":        "markdown",
					"content":    text,
					"element_id": "markdown_main",
				},
			},
		},
	}
	if ty == "thought" {
		card["header"] = map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": "思考",
			},
			"template": "blue",
		}
	}
	data, _ := json.Marshal(card)
	return string(data)
}

// doUpdate performs the actual API call
func (s *StreamCard) doUpdate(ctx context.Context) {
	s.mu.Lock()
	text := s.fullText
	cardId := s.cardId

	if cardId == nil {
		// First text received, create and send the card
		s.sequence = 0
		newCardId, err := s.client.CreateCard(ctx, streamingCard(s.CardType, text))
		if err != nil {
			logger.Warn("Failed to create streaming card", "err", err)
			s.mu.Unlock()
			return
		}
		_, err = s.client.SendInteractiveCardById(ctx, s.chatId, newCardId)
		if err != nil {
			logger.Warn("Failed to send streaming card", "err", err)
			s.mu.Unlock()
			return
		}
		s.cardId = &newCardId
		cardId = s.cardId
	}
	// Update existing card
	s.sequence++
	seq := s.sequence
	s.mu.Unlock()
	// logger.Debug("update card", "text", text, "seq", seq)
	err := s.client.UpdateCardElement(ctx, *cardId, "markdown_main", text, seq)
	if err != nil {
		logger.Warn("Failed to update streaming card", "err", err)
	}

}

func streamingCardEndSetting() string {
	card := map[string]any{
		"config": map[string]any{
			"streaming_mode": false,
		},
	}
	data, _ := json.Marshal(card)
	return string(data)
}
// endStreaming ends the streaming mode by updating the card settings
func (s *StreamCard) endStreaming(ctx context.Context) {
	s.mu.Lock()
	cardId := s.cardId

	if cardId == nil {
		s.mu.Unlock()
		return
	}
	s.sequence++
	seq := s.sequence
	s.mu.Unlock()
	err := s.client.UpdateCard(ctx, *cardId, streamingCardEndSetting(), seq)
	if err != nil {
		logger.Warn("Failed to end streaming card", "err", err)
	}
}

func (s *StreamCard) WriteChunk(text string) {
	s.mu.Lock()
	s.fullText += text
	s.mu.Unlock()
	select {
	case s.update <- false:
	default:
	}
}

func (s *StreamCard) Close() {
	s.update <- true
	close(s.update)
	s.wg.Wait()
	s.endStreaming(s.ctx)
}
