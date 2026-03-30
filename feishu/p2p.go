package feishu

import (
	"context"
	"fmt"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// SendPrivateMessage sends a private message to a user
func (c *Client) SendPrivateMessage(ctx context.Context, openID, content, msgType string) error {
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeOpenId).
		Body(&larkim.CreateMessageReqBody{
			ReceiveId:   larkcore.StringPtr(openID),
			MsgType:     larkcore.StringPtr(msgType),
			Content:     larkcore.StringPtr(content),
		}).
		Build()

	resp, err := c.client.Im.Message.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send private message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to send private message: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	return nil
}

// SendInteractiveCardToUser sends an interactive card to a user's private chat
func (c *Client) SendInteractiveCardToUser(ctx context.Context, openID, cardContent string) error {
	return c.SendPrivateMessage(ctx, openID, cardContent, "interactive")
}
