package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/ri-char/lark-acp/logger"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkcardkit "github.com/larksuite/oapi-sdk-go/v3/service/cardkit/v1"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

var client *lark.Client

func Init(appID, appSecret string) {
	client = lark.NewClient(appID, appSecret,
		lark.WithLogger(logger.NewLarkLogger(slog.LevelInfo)),
	)
}

// SendMessage sends a text message to a chat
func SendMessage(ctx context.Context, chatID, content string) error {
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

	resp, err := client.Im.Message.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to send message: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	return nil
}

// SendInteractiveCard sends an interactive card message
func SendInteractiveCard(ctx context.Context, chatID, cardContent string) (*string, error) {
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(&larkim.CreateMessageReqBody{
			ReceiveId: larkcore.StringPtr(chatID),
			MsgType:   larkcore.StringPtr("interactive"),
			Content:   larkcore.StringPtr(cardContent),
		}).
		Build()

	resp, err := client.Im.Message.Create(ctx, req)
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
func UpdateInteractiveCard(ctx context.Context, cardContent, msgId string) error {
	req := larkim.NewPatchMessageReqBuilder().
		MessageId(msgId).
		Body(&larkim.PatchMessageReqBody{
			Content: larkcore.StringPtr(cardContent),
		}).
		Build()

	resp, err := client.Im.Message.Patch(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to update card: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to update card: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func SendOrUpdateInteractiveCard(ctx context.Context, chatID, cardContent string, msgId *string) (*string, error) {
	if msgId == nil {
		return SendInteractiveCard(ctx, chatID, cardContent)
	} else {
		err := UpdateInteractiveCard(ctx, cardContent, *msgId)
		return msgId, err
	}
}

func PutTopNotice(ctx context.Context, chatID, msgId string) error {
	resp, err := client.Im.ChatTopNotice.PutTopNotice(ctx,
		larkim.NewPutTopNoticeChatTopNoticeReqBuilder().
			ChatId(chatID).
			Body(&larkim.PutTopNoticeChatTopNoticeReqBody{
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
func CreateGroup(ctx context.Context, name string, userID string) (string, error) {
	req := larkim.NewCreateChatReqBuilder().
		UserIdType(larkim.UserIdTypeOpenId).
		SetBotManager(true).
		Body(&larkim.CreateChatReqBody{
			Name:       larkcore.StringPtr(name),
			OwnerId:    larkcore.StringPtr(userID),
			UserIdList: []string{userID},
		}).
		Build()

	resp, err := client.Im.Chat.Create(ctx, req)
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

func GetGroupShareLink(ctx context.Context, chatID string) (*larkim.LinkChatResp, error) {
	req := larkim.NewLinkChatReqBuilder().
		Body(&larkim.LinkChatReqBody{
			ValidityPeriod: larkcore.StringPtr("permanently"),
		}).
		ChatId(chatID).
		Build()
	resp, err := client.Im.Chat.Link(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get share link for group: %w", err)
	}
	return resp, nil
}

func DeleteGroup(ctx context.Context, chatID string) error {
	req := larkim.NewDeleteChatReqBuilder().
		ChatId(chatID).
		Build()
	resp, err := client.Im.Chat.Delete(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to delete group: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func CreateCard(ctx context.Context, cardContent string) (string, error) {
	req := larkcardkit.NewCreateCardReqBuilder().
		Body(&larkcardkit.CreateCardReqBody{
			Type: larkcore.StringPtr("card_json"),
			Data: larkcore.StringPtr(cardContent),
		}).
		Build()

	resp, err := client.Cardkit.V1.Card.Create(ctx, req)
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

func UpdateCardElement(ctx context.Context, cardId string, elementId string, content string, sequence int) error {
	req := larkcardkit.NewContentCardElementReqBuilder().
		CardId(cardId).
		ElementId(elementId).
		Body(&larkcardkit.ContentCardElementReqBody{
			Content:  larkcore.StringPtr(content),
			Sequence: larkcore.IntPtr(sequence),
		}).
		Build()

	resp, err := client.Cardkit.V1.CardElement.Content(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to UpdateCardElement: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to UpdateCardElement: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func UpdateCard(ctx context.Context, cardId string, settings string, sequence int) error {
	req := larkcardkit.NewSettingsCardReqBuilder().
		CardId(cardId).
		Body(&larkcardkit.SettingsCardReqBody{
			Settings: larkcore.StringPtr(settings),
			Sequence: larkcore.IntPtr(sequence),
		}).
		Build()

	resp, err := client.Cardkit.V1.Card.Settings(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to UpdateCard: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to UpdateCard: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func SendInteractiveCardById(ctx context.Context, chatID, cardId string) (*string, error) {
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
	return SendInteractiveCard(ctx, chatID, string(contentBytes))
}

// GetClient returns the underlying lark client
func GetClient() *lark.Client {
	return client
}

func PinMessage(ctx context.Context, msgId string) error {
	req := larkim.NewCreatePinReqBuilder().
		Body(larkim.NewCreatePinReqBodyBuilder().
			MessageId(msgId).
			Build()).
		Build()
	resp, err := client.Im.Pin.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to pin message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to pin message: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func SendOrUpdatePinCard(ctx context.Context, cardContent, chatId string, cardId **string) {
	if *cardId != nil {
		if err := UpdateInteractiveCard(ctx, cardContent, **cardId); err != nil {
			logger.Warnf("Failed to update pin card: %v", err)
		}
	} else {
		msgID, err := SendInteractiveCard(ctx, chatId, cardContent)
		if err != nil {
			logger.Warnf("Failed to send pin card: %v", err)
			return
		}
		*cardId = msgID
		if msgID != nil {
			err := PinMessage(ctx, *msgID)
			if err != nil {
				logger.Warnf("Failed to pin message: %v", err)
			}
		}
	}
}

func SendOrUpdateTopNoticeCard(ctx context.Context, cardContent, chatId string, cardId **string) {
	if *cardId != nil {
		if err := UpdateInteractiveCard(ctx, cardContent, **cardId); err != nil {
			logger.Warnf("Failed to update pin card: %v", err)
		}
	} else {
		msgID, err := SendInteractiveCard(ctx, chatId, cardContent)
		if err != nil {
			logger.Warnf("Failed to send pin card: %v", err)
			return
		}
		*cardId = msgID
		if msgID != nil {
			err := PutTopNotice(ctx, chatId, *msgID)
			if err != nil {
				logger.Warnf("Failed to pin message: %v", err)
			}
		}

	}
}

// SendPrivateMessage sends a private message to a user
func SendPrivateMessage(ctx context.Context, openID, content, msgType string) error {
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeOpenId).
		Body(&larkim.CreateMessageReqBody{
			ReceiveId: larkcore.StringPtr(openID),
			MsgType:   larkcore.StringPtr(msgType),
			Content:   larkcore.StringPtr(content),
		}).
		Build()

	resp, err := client.Im.Message.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send private message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to send private message: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	return nil
}

// SendInteractiveCardToUser sends an interactive card to a user's private chat
func SendInteractiveCardToUser(ctx context.Context, openID, cardContent string) error {
	return SendPrivateMessage(ctx, openID, cardContent, "interactive")
}

func GetMessage(ctx context.Context, msgId string) (*larkim.Message, error) {
	req := larkim.NewGetMessageReqBuilder().
		MessageId(msgId).
		Build()

	resp, err := client.Im.Message.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	if !resp.Success() {
		return nil, fmt.Errorf("failed to get message: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	if len(resp.Data.Items) == 0 {
		return nil, fmt.Errorf("failed to get message: item is empty")
	}
	msg := resp.Data.Items[0]
	return msg, nil
}

func GetImageInMessage(ctx context.Context, imageKey string, msgId string) (io.Reader, string, error) {
	req := larkim.NewGetMessageResourceReqBuilder().
		FileKey(imageKey).
		MessageId(msgId).
		Type("image").
		Build()

	resp, err := client.Im.MessageResource.Get(ctx, req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get image: %w", err)
	}
	contentType := resp.Header.Get("Content-Type")

	if !resp.Success() {
		return nil, "", fmt.Errorf("failed to get image: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	return resp.File, contentType, nil
}
func UploadImage(ctx context.Context, data io.Reader) (string, error) {
	req := larkim.NewCreateImageReqBuilder().
		Body(larkim.NewCreateImageReqBodyBuilder().
			ImageType(`message`).
			Image(data).
			Build()).
		Build()

	// 发起请求
	resp, err := client.Im.V1.Image.Create(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("failed to upload image: %w", err)
	}
	if !resp.Success() {
		return "", fmt.Errorf("failed to upload image: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data.ImageKey == nil {
		return "", fmt.Errorf("failed to upload image")
	}
	return *resp.Data.ImageKey, nil
}
