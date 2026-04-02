package components

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/coder/acp-go-sdk"
	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/ri-char/lark-acp/feishu"
)

type ToolCallCard struct {
	content             []acpsdk.ToolCallContent
	kind                acpsdk.ToolKind
	locations           []acpsdk.ToolCallLocation
	status              acpsdk.ToolCallStatus
	title               string
	rawInput            any
	permission          []acpsdk.PermissionOption
	permissionSelected  *string
	permissionCancel    bool
	permissionRequestID string
	toolCallId          string
	infoMu              sync.RWMutex

	msgId *string
	msgMu sync.Mutex
}

func NewToolCallCard() *ToolCallCard {
	return &ToolCallCard{}
}

func (c *ToolCallCard) UpdateBySessionUpdateToolCall(ToolCall *acpsdk.SessionUpdateToolCall) {
	c.infoMu.Lock()
	defer c.infoMu.Unlock()
	c.content = ToolCall.Content
	c.locations = ToolCall.Locations
	c.title = ToolCall.Title
	c.status = ToolCall.Status
	c.kind = ToolCall.Kind
	c.rawInput = ToolCall.RawInput
	c.toolCallId = string(ToolCall.ToolCallId)
}

func (c *ToolCallCard) UpdateBySessionToolCallUpdate(ToolCallUpdate *acpsdk.SessionToolCallUpdate) {
	c.infoMu.Lock()
	defer c.infoMu.Unlock()
	if len(ToolCallUpdate.Content) > 0 {
		c.content = ToolCallUpdate.Content
	}
	if ToolCallUpdate.Locations != nil {
		c.locations = ToolCallUpdate.Locations
	}
	if ToolCallUpdate.Title != nil {
		c.title = *ToolCallUpdate.Title
	}
	if ToolCallUpdate.Status != nil {
		c.status = *ToolCallUpdate.Status
	}
	if ToolCallUpdate.Kind != nil {
		c.kind = *ToolCallUpdate.Kind
	}
	if ToolCallUpdate.RawInput != nil {
		c.rawInput = ToolCallUpdate.RawInput
	}
	c.toolCallId = string(ToolCallUpdate.ToolCallId)
}

func (c *ToolCallCard) UpdateByToolCallUpdate(ToolCallUpdate *acpsdk.ToolCallUpdate) {
	c.infoMu.Lock()
	defer c.infoMu.Unlock()
	if len(ToolCallUpdate.Content) > 0 {
		c.content = ToolCallUpdate.Content
	}
	if ToolCallUpdate.Locations != nil {
		c.locations = ToolCallUpdate.Locations
	}
	if ToolCallUpdate.Title != nil {
		c.title = *ToolCallUpdate.Title
	}
	if ToolCallUpdate.Status != nil {
		c.status = *ToolCallUpdate.Status
	}
	if ToolCallUpdate.Kind != nil {
		c.kind = *ToolCallUpdate.Kind
	}
	if ToolCallUpdate.RawInput != nil {
		c.rawInput = ToolCallUpdate.RawInput
	}
	c.toolCallId = string(ToolCallUpdate.ToolCallId)
}

func (c *ToolCallCard) GetDescMarkdown() string {
	c.infoMu.RLock()
	defer c.infoMu.RUnlock()
	var contentBuilder strings.Builder

	printFileLocation := func() {
		if len(c.locations) > 0 {
			contentBuilder.WriteString("**文件:**\n")
			for _, loc := range c.locations {
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
	switch c.kind {
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
		contentBuilder.WriteString(string(c.kind))
	}

	if c.rawInput != nil {
		data, err := json.MarshalIndent(c.rawInput, "", "  ")
		if err == nil {
			fmt.Fprintf(&contentBuilder, "**参数:**\n```\n%s\n```\n", data)
		}
	}

	return contentBuilder.String()
}

func (c *ToolCallCard) CetCardStructure() any {
	c.infoMu.RLock()
	defer c.infoMu.RUnlock()
	// 状态颜色映射
	var templateColor string
	switch c.status {
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
	title := "🔧 " + c.title

	if len(c.permission) > 0 {
		if c.permissionSelected == nil && !c.permissionCancel {
			title = "🔐 权限请求"
			templateColor = "grey"
			if len(c.title) > 0 {
				title = "🔐 权限请求: " + c.title
			}
			content = append(content, map[string]any{
				"tag": "div",
				"text": map[string]any{
					"tag":     "lark_md",
					"content": "**请选择操作：**",
				},
			})
			for _, opt := range c.permission {
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
					"name": "permission_" + c.permissionRequestID + "_0_" + string(opt.OptionId),
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
								"request_id": c.permissionRequestID,
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
				"name": "permission_" + c.permissionRequestID + "_1_cancel",
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
							"request_id": c.permissionRequestID,
							"cancel":     true,
						},
					},
				},
			})
		} else if c.permissionSelected != nil {
			for _, opt := range c.permission {
				if string(opt.OptionId) != *c.permissionSelected {
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
		} else if c.permissionCancel {
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

func (c *ToolCallCard) UpdateFeishu(ctx context.Context, chatId string) error {
	cardByte, _ := json.Marshal(c.CetCardStructure())
	card := string(cardByte)

	c.msgMu.Lock()
	defer c.msgMu.Unlock()
	msgIdPtr, err := feishu.SendOrUpdateInteractiveCard(context.Background(), chatId, card, c.msgId)
	if err != nil {
		return err
	}
	if msgIdPtr != nil {
		c.msgId = msgIdPtr
	}
	return nil
}

func (c *ToolCallCard) SelectPermission(optionId string) {
	c.infoMu.Lock()
	defer c.infoMu.Unlock()
	c.permissionSelected = &optionId
}

func (c *ToolCallCard) CancelPermission() {
	c.infoMu.Lock()
	defer c.infoMu.Unlock()
	c.permissionCancel = true
}

func (c* ToolCallCard) SetPermissionRequestID(requestId string) {
	c.infoMu.Lock()
	defer c.infoMu.Unlock()
	c.permissionRequestID = requestId
}

func (c* ToolCallCard) SetPermissionList(options []acpsdk.PermissionOption) {
	c.infoMu.Lock()
	defer c.infoMu.Unlock()
	c.permission = options
}