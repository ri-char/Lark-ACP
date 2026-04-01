package components

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coder/acp-go-sdk"
	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/ri-char/lark-acp/feishu"
)

type ToolCallCard struct {
	Content             []acpsdk.ToolCallContent  `json:"content,omitempty"`
	Kind                acpsdk.ToolKind           `json:"kind,omitempty"`
	Locations           []acpsdk.ToolCallLocation `json:"locations,omitempty"`
	Status              acpsdk.ToolCallStatus     `json:"status,omitempty"`
	Title               string                    `json:"title"`
	MsgId               *string                   `json:"msgId,omitempty"`
	RawInput            any
	Permission          []acpsdk.PermissionOption
	PermissionSelected  *string
	PermissionCancel    bool
	PermissionRequestID string
	ToolCallId          string
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
	c.RawInput = ToolCall.RawInput
	c.ToolCallId = string(ToolCall.ToolCallId)
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
	if ToolCallUpdate.RawInput != nil {
		c.RawInput = ToolCallUpdate.RawInput
	}
	c.ToolCallId = string(ToolCallUpdate.ToolCallId)
}
func (c *ToolCallCard) UpdateByToolCallUpdate(ToolCallUpdate *acpsdk.ToolCallUpdate) {
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
	if ToolCallUpdate.RawInput != nil {
		c.RawInput = ToolCallUpdate.RawInput
	}
	c.ToolCallId = string(ToolCallUpdate.ToolCallId)
}
func (c *ToolCallCard) GetDescMarkdown() string {
	var contentBuilder strings.Builder

	printFileLocation := func() {
		if len(c.Locations) > 0 {
			contentBuilder.WriteString("**文件:**\n")
			for _, loc := range c.Locations {
				if loc.Line != nil {
					fmt.Fprintf(&contentBuilder, "- `%s:%d`\n", loc.Path, *loc.Line)
				} else {
					fmt.Fprintf(&contentBuilder, "- `%s`\n", loc.Path)
				}
			}
			contentBuilder.WriteString("\n")
		}
	}

	contentBuilder.WriteString("**类型:** ")
	switch c.Kind {
	case acp.ToolKindRead:
		contentBuilder.WriteString("读取文件\n")
		printFileLocation()
	case acp.ToolKindEdit:
		contentBuilder.WriteString("编辑文件\n")
		printFileLocation()
	case acp.ToolKindDelete:
		contentBuilder.WriteString("删除文件\n")
		printFileLocation()
	case acp.ToolKindMove:
		contentBuilder.WriteString("移动文件\n")
		printFileLocation()
	case acp.ToolKindSearch:
		contentBuilder.WriteString("搜索内容\n")
	case acp.ToolKindExecute:
		contentBuilder.WriteString("执行命令\n")
	case acp.ToolKindThink:
		contentBuilder.WriteString("思考\n")
	case acp.ToolKindFetch:
		contentBuilder.WriteString("网络获取\n")
	case acp.ToolKindSwitchMode:
		contentBuilder.WriteString("切换模式\n")
	case acp.ToolKindOther:
		contentBuilder.WriteString("其他操作\n")
	default:
		contentBuilder.WriteString(string(c.Kind))
	}

	if c.RawInput != nil {
		data, err := json.MarshalIndent(c.RawInput, "", "  ")
		if err == nil {
			fmt.Fprintf(&contentBuilder, "**参数:**\n```\n%s\n```\n", data)
		}
	}

	return contentBuilder.String()
}

// ToolCallCard creates a card for displaying tool call status
func (c *ToolCallCard) CetCardStructure() any {
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

	content := []map[string]any{
		{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": c.GetDescMarkdown(),
			},
		},
	}
	title := "🔧 " + c.Title

	if len(c.Permission) > 0 {
		if c.PermissionSelected == nil && !c.PermissionCancel {
			title = "🔐 权限请求"
			templateColor = "grey"
			if len(c.Title) > 0 {
				title = "🔐 权限请求: " + c.Title
			}
			content = append(content, map[string]any{
				"tag": "div",
				"text": map[string]any{
					"tag":     "lark_md",
					"content": "**请选择操作：**",
				},
			})
			for _, opt := range c.Permission {
				// 根据类型选择按钮样式
				var buttonType string
				switch opt.Kind {
				case acpsdk.PermissionOptionKindAllowOnce, acpsdk.PermissionOptionKindAllowAlways:
					buttonType = "primary"
				default:
					buttonType = "default"
				}
				btnCard := map[string]any{
					"tag":  "button",
					"name": "permission_" + c.PermissionRequestID + "_0_" + string(opt.OptionId),
					"text": map[string]any{
						"tag":     "plain_text",
						"content": opt.Name,
					},
					"type": buttonType,
					"behaviors": []map[string]any{
						{
							"type": "callback",
							"value": map[string]any{
								"action":     "permission",
								"request_id": c.PermissionRequestID,
								"option_id":  string(opt.OptionId),
								"cancel":     false,
							},
						},
					},
				}
				switch opt.Kind {
				case acp.PermissionOptionKindRejectOnce:
					btnCard["confirm"] = map[string]any{
						"text": map[string]any{
							"tag":     "plain_text",
							"content": "确认本次拒绝执行吗？",
						},
					}
				case acp.PermissionOptionKindRejectAlways:
					btnCard["confirm"] = map[string]any{
						"text": map[string]any{
							"tag":     "plain_text",
							"content": "确认永远拒绝执行吗？",
						},
					}
				}

				content = append(content, btnCard)
			}
			content = append(content, map[string]any{
				"tag":  "button",
				"name": "permission_" + c.PermissionRequestID + "_1_cancel",
				"text": map[string]any{
					"tag":     "plain_text",
					"content": "取消",
				},
				"type": "danger",
				"confirm": map[string]any{
					"text": map[string]any{
						"tag":     "plain_text",
						"content": "确认取消吗？",
					},
				},
				"behaviors": []map[string]any{
					{
						"type": "callback",
						"value": map[string]any{
							"action":     "permission",
							"request_id": c.PermissionRequestID,
							"cancel":     true,
						},
					},
				},
			})
		} else if c.PermissionSelected != nil {
			for _, opt := range c.Permission {
				if string(opt.OptionId) != *c.PermissionSelected {
					continue
				}
				if opt.Kind != acpsdk.PermissionOptionKindAllowOnce && opt.Kind != acpsdk.PermissionOptionKindAllowAlways {
					templateColor = "red"
				}
				content = append(content, map[string]any{
					"tag":  "button",
					"name": "permission_" + "_0_" + string(opt.OptionId),
					"text": map[string]any{
						"tag":     "plain_text",
						"content": opt.Name,
					},
					"type":     "primary",
					"disabled": true,
				})
				break
			}
		} else if c.PermissionCancel {
			title += " (已取消)"
			content = append(content, map[string]any{
				"tag":  "button",
				"name": "permission_" + "_1_cancel",
				"text": map[string]any{
					"tag":     "plain_text",
					"content": "取消",
				},
				"type":     "primary",
				"disabled": true,
			})
		}
	}

	card := map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": title,
			},
			"template": templateColor,
		},
		"body": map[string]any{
			"elements": content,
		},
	}
	return card
}

func (c *ToolCallCard) UpdateFeishu(ctx context.Context, client *feishu.Client, chatId string) error {
	cardByte, _ := json.Marshal(c.CetCardStructure())
	card := string(cardByte)
	msgIdPtr, err := client.SendOrUpdateInteractiveCard(context.Background(), chatId, card, c.MsgId)
	if err != nil {
		return err
	}
	if msgIdPtr != nil {
		c.MsgId = msgIdPtr
	}
	return nil
}
