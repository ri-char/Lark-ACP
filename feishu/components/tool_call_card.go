package components

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/ri-char/lark-acp/feishu"
	"github.com/ri-char/lark-acp/logger"
)

type ToolCallCard struct {
	Content   []acpsdk.ToolCallContent  `json:"content,omitempty"`
	Kind      acpsdk.ToolKind           `json:"kind,omitempty"`
	Locations []acpsdk.ToolCallLocation `json:"locations,omitempty"`
	Status    acpsdk.ToolCallStatus     `json:"status,omitempty"`
	Title     string                    `json:"title"`
	MsgId     *string                   `json:"msgId,omitempty"`
}

func NewToolCallCard() *ToolCallCard {
	return &ToolCallCard{}
}

func (c *ToolCallCard) UpdateBySessionUpdateToolCall(ToolCall *acpsdk.SessionUpdateToolCall) {
	c.Content = ToolCall.Content
	c.Locations = ToolCall.Locations
	c.Title = ToolCall.Title
	c.Status = ToolCall.Status
	c.Kind = ToolCall.Kind
}

func (c *ToolCallCard) UpdateBySessionToolCallUpdate(ToolCallUpdate *acpsdk.SessionToolCallUpdate) {
	if len(ToolCallUpdate.Content) > 0 {
		c.Content = ToolCallUpdate.Content
	}
	if ToolCallUpdate.Locations != nil {
		c.Locations = ToolCallUpdate.Locations
	}
	if ToolCallUpdate.Title != nil {
		c.Title = *ToolCallUpdate.Title
	}
	if ToolCallUpdate.Status != nil {
		c.Status = *ToolCallUpdate.Status
	}
	if ToolCallUpdate.Kind != nil {
		c.Kind = *ToolCallUpdate.Kind
	}
}

// ToolCallCard creates a card for displaying tool call status
func (c *ToolCallCard) getCardString() string {
	// 状态颜色映射
	var templateColor string
	switch c.Status {
	case acpsdk.ToolCallStatusInProgress:
		templateColor = "blue"
	case acpsdk.ToolCallStatusCompleted:
		templateColor = "green"
	case acpsdk.ToolCallStatusFailed:
		templateColor = "red"
	case acpsdk.ToolCallStatusPending:
		templateColor = "grey"
	default:
		templateColor = "grey"
	}

	// 构建内容
	var contentBuilder strings.Builder
	if len(string(c.Kind)) > 0 {
		contentBuilder.WriteString(fmt.Sprintf("**类型:** %s\n", string(c.Kind)))
	}
	// 显示文件路径
	if len(c.Locations) > 0 {
		contentBuilder.WriteString("**文件:**\n")
		for _, loc := range c.Locations {
			if loc.Line != nil {
				contentBuilder.WriteString(fmt.Sprintf("- `%s:%d`\n", loc.Path, *loc.Line))
			} else {
				contentBuilder.WriteString(fmt.Sprintf("- `%s`\n", loc.Path))
			}
		}
		contentBuilder.WriteString("\n")
	}

	// 显示内容（如果有 diff 或文本）
	// if len(c.Content) > 0 {
	// 	contentBuilder.WriteString("**内容:**\n")
	// 	for _, c := range c.Content {
	// 		if c.Diff != nil {
	// 			contentBuilder.WriteString(fmt.Sprintf("```diff\n%s\n```\n", c.Diff.NewText))
	// 		} else if c.Content != nil && c.Content.Content.Text != nil {
	// 			contentBuilder.WriteString(fmt.Sprintf("%s\n", c.Content.Content.Text.Text))
	// 		}
	// 	}
	// }

	card := map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": "🔧 " + c.Title,
			},
			"template": templateColor,
		},
		"body": map[string]any{
			"elements": []map[string]any{
				{
					"tag": "div",
					"text": map[string]any{
						"tag":     "lark_md",
						"content": contentBuilder.String(),
					},
				},
			},
		},
	}
	data, _ := json.Marshal(card)
	return string(data)
}

func (c *ToolCallCard) UpdateFeishu(ctx context.Context, client *feishu.Client, chatId string) {
	card := c.getCardString()
	msgIdPtr, err := client.SendOrUpdateInteractiveCard(context.Background(), chatId, card, c.MsgId)
	if err != nil {
		logger.Debugf("Failed to send tool call card to Feishu: %v", err)
	}
	if msgIdPtr != nil {
		c.MsgId = msgIdPtr
	}
}
