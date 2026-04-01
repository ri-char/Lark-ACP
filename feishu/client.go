package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ri-char/lark-acp/logger"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkcardkit "github.com/larksuite/oapi-sdk-go/v3/service/cardkit/v1"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type Client struct {
	client    *lark.Client
	appID     string
	appSecret string
}

func New(appID, appSecret string) (*Client, error) {
	cli := lark.NewClient(appID, appSecret,
		lark.WithLogger(logger.NewLarkLogger(slog.LevelInfo)),
	)
	return &Client{
		client:    cli,
		appID:     appID,
		appSecret: appSecret,
	}, nil
}

// SendMessage sends a text message to a chat
func (c *Client) SendMessage(ctx context.Context, chatID, content string) error {
	send_data := map[string]string{
		"text": content,
	}
	contentBytes, err := json.Marshal(send_data)
	if err != nil {
		return fmt.Errorf("failed to marshal message content: %w", err)
	}
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(&larkim.CreateMessageReqBody{
			ReceiveId: larkcore.StringPtr(chatID),
			MsgType:   larkcore.StringPtr("text"),
			Content:   larkcore.StringPtr(string(contentBytes)),
		}).
		Build()

	resp, err := c.client.Im.Message.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to send message: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	return nil
}

// SendInteractiveCard sends an interactive card message
func (c *Client) SendInteractiveCard(ctx context.Context, chatID, cardContent string) (*string, error) {
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(&larkim.CreateMessageReqBody{
			ReceiveId: larkcore.StringPtr(chatID),
			MsgType:   larkcore.StringPtr("interactive"),
			Content:   larkcore.StringPtr(cardContent),
		}).
		Build()

	resp, err := c.client.Im.Message.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send card: %w", err)
	}

	if !resp.Success() {
		return nil, fmt.Errorf("failed to send card: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.MessageId == nil {
		return nil, fmt.Errorf("empty message ID in response")
	}
	return resp.Data.MessageId, nil
}
func (c *Client) UpdateInteractiveCard(ctx context.Context, cardContent, msgId string) error {
	req := larkim.NewPatchMessageReqBuilder().
		MessageId(msgId).
		Body(&larkim.PatchMessageReqBody{
			Content: larkcore.StringPtr(cardContent),
		}).
		Build()

	resp, err := c.client.Im.Message.Patch(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to update card: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to update card: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (c *Client) SendOrUpdateInteractiveCard(ctx context.Context, chatID, cardContent string, msgId *string) (*string, error) {
	if msgId == nil {
		return c.SendInteractiveCard(ctx, chatID, cardContent)
	} else {
		err := c.UpdateInteractiveCard(ctx, cardContent, *msgId)
		return msgId, err
	}
}

func (c *Client) PutTopNotice(ctx context.Context, chatID, msgId string) error {
	resp, err := c.client.Im.ChatTopNotice.PutTopNotice(ctx, larkim.NewPutTopNoticeChatTopNoticeReqBuilder().ChatId(chatID).Body(&larkim.PutTopNoticeChatTopNoticeReqBody{
		ChatTopNotice: []*larkim.ChatTopNotice{
			{
				ActionType: larkcore.StringPtr("1"),
				MessageId:  larkcore.StringPtr(msgId),
			},
		},
	}).Build())
	if err != nil {
		return fmt.Errorf("failed to PutTopNotice: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to PutTopNotice: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

// CreateGroup creates a new group chat and returns the chat ID
func (c *Client) CreateGroup(ctx context.Context, name string, userID string) (string, error) {
	req := larkim.NewCreateChatReqBuilder().
		UserIdType(larkim.UserIdTypeOpenId).
		SetBotManager(true).
		Body(&larkim.CreateChatReqBody{
			Name:       larkcore.StringPtr(name),
			OwnerId:    larkcore.StringPtr(userID),
			UserIdList: []string{userID},
		}).
		Build()

	resp, err := c.client.Im.Chat.Create(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to create group: %w", err)
	}

	if !resp.Success() {
		return "", fmt.Errorf("failed to create group: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	if resp.Data == nil || resp.Data.ChatId == nil {
		return "", fmt.Errorf("empty chat ID in response")
	}

	return *resp.Data.ChatId, nil
}

func (c *Client) GetGroupShareLink(ctx context.Context, chatID string) (*larkim.LinkChatResp, error) {
	req := larkim.NewLinkChatReqBuilder().
		Body(&larkim.LinkChatReqBody{
			ValidityPeriod: larkcore.StringPtr("permanently"),
		}).
		ChatId(chatID).
		Build()
	resp, err := c.client.Im.Chat.Link(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get share link for group: %w", err)
	}
	return resp, nil
}

func (c *Client) DeleteGroup(ctx context.Context, chatID string) error {
	req := larkim.NewDeleteChatReqBuilder().
		ChatId(chatID).
		Build()
	resp, err := c.client.Im.Chat.Delete(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to delete group: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (c *Client) CreateCard(ctx context.Context, cardContent string) (string, error) {
	req := larkcardkit.NewCreateCardReqBuilder().
		Body(&larkcardkit.CreateCardReqBody{
			Type: larkcore.StringPtr("card_json"),
			Data: larkcore.StringPtr(cardContent),
		}).
		Build()

	resp, err := c.client.Cardkit.V1.Card.Create(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to create card: %w", err)
	}

	if !resp.Success() {
		return "", fmt.Errorf("failed to create card: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.CardId == nil {
		return "", fmt.Errorf("empty message ID in response")
	}
	return *resp.Data.CardId, nil
}

func (c *Client) UpdateCardElement(ctx context.Context, cardId string, elementId string, content string, sequence int) error {
	req := larkcardkit.NewContentCardElementReqBuilder().
		CardId(cardId).
		ElementId(elementId).
		Body(&larkcardkit.ContentCardElementReqBody{
			Content:  larkcore.StringPtr(content),
			Sequence: larkcore.IntPtr(sequence),
		}).
		Build()

	resp, err := c.client.Cardkit.V1.CardElement.Content(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to UpdateCardElement: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to UpdateCardElement: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (c *Client) UpdateCard(ctx context.Context, cardId string, settings string, sequence int) error {
	req := larkcardkit.NewSettingsCardReqBuilder().
		CardId(cardId).
		Body(&larkcardkit.SettingsCardReqBody{
			Settings: larkcore.StringPtr(settings),
			Sequence: larkcore.IntPtr(sequence),
		}).
		Build()

	resp, err := c.client.Cardkit.V1.Card.Settings(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to UpdateCard: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to UpdateCard: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (c *Client) SendInteractiveCardById(ctx context.Context, chatID, cardId string) (*string, error) {
	data := map[string]any{
		"type": "card",
		"data": map[string]any{
			"card_id": cardId,
		},
	}
	contentBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message content: %w", err)
	}
	return c.SendInteractiveCard(ctx, chatID, string(contentBytes))
}

// GetClient returns the underlying lark client
func (c *Client) GetClient() *lark.Client {
	return c.client
}

func (c *Client) PinMessage(ctx context.Context, msgId string) error {
	req := larkim.NewCreatePinReqBuilder().
		Body(larkim.NewCreatePinReqBodyBuilder().
			MessageId(msgId).
			Build()).
		Build()
	resp, err := c.client.Im.Pin.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to pin message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to pin message: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (c *Client) SendOrUpdatePinCard(ctx context.Context, cardContent, chatId string, cardId *string) {
	if cardId != nil {
		if err := c.UpdateInteractiveCard(ctx, cardContent, *cardId); err != nil {
			logger.Debugf("Failed to update pin card: %v", err)
		}
	} else {
		msgID, err := c.SendInteractiveCard(ctx, chatId, cardContent)
		if err != nil {
			logger.Debugf("Failed to send pin card: %v", err)
			return
		}
		cardId = msgID
		if msgID != nil {
			err := c.PinMessage(ctx, *msgID)
			if err != nil {
				logger.Debugf("Failed to pin message: %v", err)
			}
		}

	}
}
