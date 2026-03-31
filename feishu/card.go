package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/coder/acp-go-sdk"
	acpsdk "github.com/coder/acp-go-sdk"
	larkcard "github.com/larksuite/oapi-sdk-go/v3/card"
	"github.com/ri-char/lark-acp/session"
)

// PathInputCard creates a card for path input
func AgentSelectionCard(agents []string) string {
	// 构建 agent 选项列表
	options := make([]map[string]any, len(agents))
	for i, agent := range agents {
		options[i] = map[string]any{
			"text": map[string]any{
				"tag":     "plain_text",
				"content": agent,
			},
			"value": agent,
		}
	}
	defaultPath, err := os.Getwd()
	if err != nil {
		defaultPath = "/home/user"
	}

	// JSON 2.0 卡片结构
	cardV2 := map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": "创建新会话",
			},
			"template": "blue",
		},
		"body": map[string]any{
			"elements": []map[string]any{
				{
					"tag":  "form",
					"name": "path_form",
					"elements": []map[string]any{
						{
							"tag":                "column_set",
							"horizontal_spacing": "8px",
							"horizontal_align":   "left",
							"columns": []map[string]any{
								{
									"tag":    "column",
									"width":  "weighted",
									"weight": 1,
									"elements": []map[string]any{
										{
											"tag":        "markdown",
											"content":    "**Agent类型**<font color='red'>*</font>",
											"text_align": "left",
										},
									},
								},
								{
									"tag":    "column",
									"width":  "weighted",
									"weight": 4,
									"elements": []map[string]any{
										{
											"tag":           "select_static",
											"name":          "agent_type",
											"required":      true,
											"initial_index": 1,
											"options":       options,
											"width":         "fill",
										},
									},
								},
							},
						},
						{
							"tag":                "column_set",
							"horizontal_spacing": "8px",
							"horizontal_align":   "left",
							"columns": []map[string]any{
								{
									"tag":    "column",
									"width":  "weighted",
									"weight": 1,
									"elements": []map[string]any{
										{
											"tag":        "markdown",
											"content":    "**工作路径**<font color='red'>*</font>",
											"text_align": "left",
										},
									},
								},
								{
									"tag":    "column",
									"width":  "weighted",
									"weight": 4,
									"elements": []map[string]any{
										{
											"tag":      "input",
											"name":     "path_input",
											"required": true,
											"placeholder": map[string]any{
												"tag":     "plain_text",
												"content": "Agent运行的绝对路径",
											},
											"default_value": defaultPath,
											"width":         "fill",
										},
									},
								},
							},
						},
						{
							"tag":                "column_set",
							"horizontal_spacing": "8px",
							"horizontal_align":   "right",
							"columns": []map[string]any{
								{
									"tag":   "column",
									"width": "auto",
									"elements": []map[string]any{
										{
											"tag":  "button",
											"name": "new_session_form",
											"text": map[string]any{
												"tag":     "plain_text",
												"content": "提交",
											},
											"type":             "primary_filled",
											"form_action_type": "submit",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	data, _ := json.Marshal(cardV2)
	return string(data)
}

// PathInputCard creates a card for path input
func LoadSessionAgentSelectionCard(agents []string) string {
	// 构建 agent 选项列表
	options := make([]map[string]any, len(agents))
	for i, agent := range agents {
		options[i] = map[string]any{
			"text": map[string]any{
				"tag":     "plain_text",
				"content": agent,
			},
			"value": agent,
		}
	}

	// JSON 2.0 卡片结构
	cardV2 := map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": "加载会话",
			},
			"template": "blue",
		},
		"body": map[string]any{
			"elements": []map[string]any{
				{
					"tag":  "form",
					"name": "path_form",
					"elements": []map[string]any{
						{
							"tag":                "column_set",
							"horizontal_spacing": "8px",
							"horizontal_align":   "left",
							"columns": []map[string]any{
								{
									"tag":    "column",
									"width":  "weighted",
									"weight": 1,
									"elements": []map[string]any{
										{
											"tag":        "markdown",
											"content":    "**Agent类型**<font color='red'>*</font>",
											"text_align": "left",
										},
									},
								},
								{
									"tag":    "column",
									"width":  "weighted",
									"weight": 4,
									"elements": []map[string]any{
										{
											"tag":           "select_static",
											"name":          "agent_type",
											"required":      true,
											"initial_index": 1,
											"options":       options,
											"width":         "fill",
										},
									},
								},
							},
						},
						{
							"tag":                "column_set",
							"horizontal_spacing": "8px",
							"horizontal_align":   "right",
							"columns": []map[string]any{
								{
									"tag":   "column",
									"width": "auto",
									"elements": []map[string]any{
										{
											"tag":  "button",
											"name": "load_session_agent",
											"text": map[string]any{
												"tag":     "plain_text",
												"content": "提交",
											},
											"type":             "primary_filled",
											"form_action_type": "submit",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	data, _ := json.Marshal(cardV2)
	return string(data)
}

// PathInputCard creates a card for path input
func LoadSessionAgentSessionCard(sessions []acp.UnstableSessionInfo, agentId string) string {
	// 构建 agent 选项列表
	options := make([]map[string]any, len(sessions))
	for i, session := range sessions {
		options[i] = map[string]any{
			"text": map[string]any{
				"tag":     "plain_text",
				"content": session.Title,
			},
			"value": session.SessionId,
		}
	}

	// JSON 2.0 卡片结构
	cardV2 := map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": "加载会话",
			}, "subtitle": map[string]any{
				"tag":     "plain_text",
				"content": "选择会话",
			},
			"template": "blue",
		},
		"body": map[string]any{
			"elements": []map[string]any{
				{
					"tag":  "form",
					"name": "path_form",
					"elements": []map[string]any{
						{
							"tag":           "select_static",
							"name":          "session_id",
							"required":      true,
							"initial_index": 1,
							"options":       options,
							"width":         "fill",
						},
						{
							"tag":  "button",
							"name": "load_session_session",
							"text": map[string]any{
								"tag":     "plain_text",
								"content": "提交",
							},
							"type":             "primary_filled",
							"form_action_type": "submit",
							"behaviors": []map[string]any{
								{
									"type": "callback",
									"value": map[string]any{
										"action":   "load_session_session",
										"agent_id": agentId,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	data, _ := json.Marshal(cardV2)
	return string(data)
}

func ErrorCard(title, message string) string {
	card := map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": title,
			},
			"template": "red",
		},
		"body": map[string]any{
			"elements": []map[string]any{
				{
					"tag": "div",
					"text": map[string]any{
						"tag":     "lark_md",
						"content": message,
					},
				},
			},
		},
	}
	data, _ := json.Marshal(card)
	return string(data)
}

func AgentSelectionFreezeCard(agentName, path string) any {
	// JSON 2.0 卡片结构
	cardV2 := map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": "创建新会话",
			},
			"template": "default",
		},
		"body": map[string]any{
			"elements": []map[string]any{
				{
					"tag":  "form",
					"name": "path_form",
					"elements": []map[string]any{
						{
							"tag":                "column_set",
							"horizontal_spacing": "8px",
							"horizontal_align":   "left",
							"columns": []map[string]any{
								{
									"tag":    "column",
									"width":  "weighted",
									"weight": 1,
									"elements": []map[string]any{
										{
											"tag":        "markdown",
											"content":    "**Agent类型**<font color='red'>*</font>",
											"text_align": "left",
										},
									},
								},
								{
									"tag":    "column",
									"width":  "weighted",
									"weight": 4,
									"elements": []map[string]any{
										{
											"tag":      "select_static",
											"name":     "agent_type",
											"disabled": true,
											"placeholder": map[string]any{
												"tag":     "plain_text",
												"content": agentName,
											},
											"width": "fill",
										},
									},
								},
							},
						},
						{
							"tag":                "column_set",
							"horizontal_spacing": "8px",
							"horizontal_align":   "left",
							"columns": []map[string]any{
								{
									"tag":    "column",
									"width":  "weighted",
									"weight": 1,
									"elements": []map[string]any{
										{
											"tag":        "markdown",
											"content":    "**工作路径**<font color='red'>*</font>",
											"text_align": "left",
										},
									},
								},
								{
									"tag":    "column",
									"width":  "weighted",
									"weight": 4,
									"elements": []map[string]any{
										{
											"tag":           "input",
											"name":          "path_input",
											"disabled":      true,
											"default_value": path,
											"width":         "fill",
										},
									},
								},
							},
						},
						{
							"tag":                "column_set",
							"horizontal_spacing": "8px",
							"horizontal_align":   "right",
							"columns": []map[string]any{
								{
									"tag":   "column",
									"width": "auto",
									"elements": []map[string]any{
										{
											"tag":      "button",
											"name":     "new_session_form",
											"disabled": true,
											"text": map[string]any{
												"tag":     "plain_text",
												"content": "提交",
											},
											"type":             "primary_filled",
											"form_action_type": "submit",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return cardV2
}

// CreateSessionConfirmCard creates a confirmation card after session creation
func NewSessionFinishCard(agentName, path, link, title string) string {
	ele := []map[string]any{
		{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": fmt.Sprintf("Agent类型: `%s`\n工作路径: `%s`", agentName, path),
			},
		},
	}
	if link != "" {
		ele = append(ele, map[string]any{
			"tag":  "button",
			"type": "primary",
			"text": map[string]any{
				"tag":     "plain_text",
				"content": "进入群聊",
			},
			"behaviors": []map[string]any{
				{
					"type":        "open_url",
					"default_url": link,
				},
			},
		})
	}

	card := map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": title,
			},
			"template": "green",
		},
		"body": map[string]any{
			"elements": ele,
		},
	}
	data, _ := json.Marshal(card)
	return string(data)
}

// ToolCallCard creates a card for displaying tool call status
func ToolCallCard(info *session.ToolCallIdInfo) string {

	// 状态颜色映射
	var templateColor string
	switch info.Status {
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
	if len(string(info.Kind)) > 0 {
		contentBuilder.WriteString(fmt.Sprintf("**类型:** %s\n", string(info.Kind)))
	}
	// 显示文件路径
	if len(info.Locations) > 0 {
		contentBuilder.WriteString("**文件:**\n")
		for _, loc := range info.Locations {
			if loc.Line != nil {
				contentBuilder.WriteString(fmt.Sprintf("- `%s:%d`\n", loc.Path, *loc.Line))
			} else {
				contentBuilder.WriteString(fmt.Sprintf("- `%s`\n", loc.Path))
			}
		}
		contentBuilder.WriteString("\n")
	}

	// 显示内容（如果有 diff 或文本）
	// if len(info.Content) > 0 {
	// 	contentBuilder.WriteString("**内容:**\n")
	// 	for _, c := range info.Content {
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
				"content": "🔧 " + info.Title,
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

// PlanCard creates a card for displaying plan entries
func PlanCard(entries []acpsdk.PlanEntry) string {
	// 构建任务列表
	var content strings.Builder

	for _, entry := range entries {
		// 状态图标
		var statusIcon string
		switch entry.Status {
		case acpsdk.PlanEntryStatusInProgress:
			statusIcon = "🔄"
		case acpsdk.PlanEntryStatusCompleted:
			statusIcon = "✅"
		default:
			statusIcon = "⏳"
		}

		// 优先级颜色
		var priorityColor string
		switch entry.Priority {
		case acpsdk.PlanEntryPriorityHigh:
			priorityColor = "<font color='red'>[高]</font>"
		case acpsdk.PlanEntryPriorityMedium:
			priorityColor = "<font color='orange'>[中]</font>"
		default:
			priorityColor = "<font color='grey'>[低]</font>"
		}

		content.WriteString(fmt.Sprintf("%s %s %s\n", statusIcon, priorityColor, entry.Content))
	}

	card := map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": "📋 执行计划",
			},
			"template": "blue",
		},
		"body": map[string]any{
			"elements": []map[string]any{
				{
					"tag": "div",
					"text": map[string]any{
						"tag":     "lark_md",
						"content": content.String(),
					},
				},
			},
		},
	}
	data, _ := json.Marshal(card)
	return string(data)
}

// NewCardActionHandler creates a handler for card actions
func NewCardActionHandler(verificationToken, encryptKey string, handler func(ctx context.Context, action *larkcard.CardAction) (any, error)) *larkcard.CardActionHandler {
	return larkcard.NewCardActionHandler(verificationToken, encryptKey, handler)
}

// PermissionCard creates a card for permission request
func PermissionCard(sessionID, requestID string, options []acpsdk.PermissionOption, toolCall acpsdk.ToolCallUpdate) string {
	// 构建 toolCall 信息
	var infoBuilder strings.Builder

	if toolCall.Kind != nil {
		infoBuilder.WriteString(fmt.Sprintf("**类型:** %s\n", string(*toolCall.Kind)))
	}

	if len(toolCall.Locations) > 0 {
		infoBuilder.WriteString("**文件:**\n")
		for _, loc := range toolCall.Locations {
			if loc.Line != nil {
				infoBuilder.WriteString(fmt.Sprintf("- `%s:%d`\n", loc.Path, *loc.Line))
			} else {
				infoBuilder.WriteString(fmt.Sprintf("- `%s`\n", loc.Path))
			}
		}
	}

	// 构建内容
	var content []map[string]any

	// 添加 toolCall 信息
	if infoBuilder.Len() > 0 {
		content = append(content, map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": infoBuilder.String(),
			},
		})
	}

	// 添加提示
	content = append(content, map[string]any{
		"tag": "div",
		"text": map[string]any{
			"tag":     "lark_md",
			"content": "**请选择操作：**",
		},
	})

	for _, opt := range options {
		// 根据类型选择按钮样式
		var buttonType string
		switch opt.Kind {
		case acpsdk.PermissionOptionKindAllowOnce, acpsdk.PermissionOptionKindAllowAlways:
			buttonType = "primary"
		default:
			buttonType = "default"
		}

		content = append(content, map[string]any{
			"tag":  "button",
			"name": "permission_" + requestID + "_0_" + string(opt.OptionId),
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
						"session_id": sessionID,
						"request_id": requestID,
						"option_id":  string(opt.OptionId),
						"cancel":     false,
					},
				},
			},
		})
	}
	content = append(content, map[string]any{
		"tag":  "button",
		"name": "permission_" + requestID + "_1_cancel",
		"text": map[string]any{
			"tag":     "plain_text",
			"content": "取消",
		},
		"type": "danger",
		"behaviors": []map[string]any{
			{
				"type": "callback",
				"value": map[string]any{
					"action":     "permission",
					"session_id": sessionID,
					"request_id": requestID,
					"cancel":     true,
				},
			},
		},
	})
	title := "🔐 权限请求"
	if toolCall.Title != nil {
		title = "🔐 权限请求: " + *toolCall.Title
	}
	card := map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": title,
			},
			"template": "orange",
		},
		"body": map[string]any{
			"elements": content,
		},
	}
	data, _ := json.Marshal(card)
	return string(data)
}
func PermissionFreezeCard(options []acpsdk.PermissionOption, toolCall acpsdk.ToolCallUpdate, cancel bool, option string) map[string]any {
	// 构建 toolCall 信息
	var infoBuilder strings.Builder

	if toolCall.Kind != nil {
		infoBuilder.WriteString(fmt.Sprintf("**类型:** %s\n", string(*toolCall.Kind)))
	}

	if len(toolCall.Locations) > 0 {
		infoBuilder.WriteString("**文件:**\n")
		for _, loc := range toolCall.Locations {
			if loc.Line != nil {
				infoBuilder.WriteString(fmt.Sprintf("- `%s:%d`\n", loc.Path, *loc.Line))
			} else {
				infoBuilder.WriteString(fmt.Sprintf("- `%s`\n", loc.Path))
			}
		}
	}
	if toolCall.RawInput != nil {
		if input, ok := toolCall.RawInput.(string); ok {
			if len(input) > 200 {
				input = input[:200] + "..."
			}
			infoBuilder.WriteString(fmt.Sprintf("**输入:**\n```\n%s\n```\n", input))
		} else if b, err := json.Marshal(toolCall.RawInput); err == nil {
			input := string(b)
			if len(input) > 200 {
				input = input[:200] + "..."
			}
			infoBuilder.WriteString(fmt.Sprintf("**输入:**\n```json\n%s\n```\n", input))
		}
	}

	// 构建内容
	var content []map[string]any

	// 添加 toolCall 信息
	if infoBuilder.Len() > 0 {
		content = append(content, map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": infoBuilder.String(),
			},
		})
	}

	// 添加提示
	content = append(content, map[string]any{
		"tag": "div",
		"text": map[string]any{
			"tag":     "lark_md",
			"content": "**请选择操作：**",
		},
	})
	color := "default"
	if !cancel {
		for _, opt := range options {
			if string(opt.OptionId) != option {
				continue
			}
			if opt.Kind == acpsdk.PermissionOptionKindAllowOnce || opt.Kind == acpsdk.PermissionOptionKindAllowAlways {
				color = "green"
			} else {
				color = "red"
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
	} else {
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
	title := "🔐 权限请求"
	if toolCall.Title != nil {
		title += ": " + *toolCall.Title
	}
	if cancel {
		title += " (已取消)"
		color = "grey"
	}
	card := map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": title,
			},
			"template": color,
		},
		"body": map[string]any{
			"elements": content,
		},
	}
	return card
}

func StreamingCard(ty string, text string) string {
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
func StreamingCardEndSetting() string {
	card := map[string]any{
		"config": map[string]any{
			"streaming_mode": false,
		},
	}
	data, _ := json.Marshal(card)
	return string(data)
}

func GroupPinHeaderCard(agent, path string, models *acpsdk.SessionModelState, modes *acpsdk.SessionModeState) string {

	elements := []map[string]any{
		{
			"tag":                "column_set",
			"horizontal_spacing": "8px",
			"horizontal_align":   "left",
			"columns": []map[string]any{
				{
					"tag":    "column",
					"width":  "weighted",
					"weight": 1,
					"elements": []map[string]any{
						{
							"tag":        "markdown",
							"content":    "**工作目录**",
							"text_align": "left",
						},
					},
				},
				{
					"tag":    "column",
					"width":  "weighted",
					"weight": 3,
					"elements": []map[string]any{
						{
							"tag":        "markdown",
							"content":    path,
							"text_align": "left",
						},
					},
				},
			},
		},
	}
	// Build models info
	if models != nil {
		modelsInfo := string(models.CurrentModelId)
		if len(models.AvailableModels) > 0 {
			for _, m := range models.AvailableModels {
				if models.CurrentModelId == m.ModelId {
					modelsInfo = string(m.Name)
					break
				}
			}
		}
		elements = append(elements, map[string]any{
			"tag":                "column_set",
			"horizontal_spacing": "8px",
			"horizontal_align":   "left",
			"columns": []map[string]any{
				{
					"tag":    "column",
					"width":  "weighted",
					"weight": 1,
					"elements": []map[string]any{
						{
							"tag":        "markdown",
							"content":    "**当前模型**",
							"text_align": "left",
						},
					},
				},
				{
					"tag":    "column",
					"width":  "weighted",
					"weight": 3,
					"elements": []map[string]any{
						{
							"tag": "div",
							"text": map[string]any{
								"tag":     "plain_text",
								"content": modelsInfo,
							},
						},
					},
				},
			},
		})
	}

	// Build modes info
	if modes != nil {
		modesInfo := string(modes.CurrentModeId)
		if len(modes.AvailableModes) > 0 {
			for _, m := range modes.AvailableModes {
				if modes.CurrentModeId == m.Id {
					modesInfo = string(m.Name)
				}
			}
		}
		elements = append(elements, map[string]any{
			"tag":                "column_set",
			"horizontal_spacing": "8px",
			"horizontal_align":   "left",
			"columns": []map[string]any{
				{
					"tag":    "column",
					"width":  "weighted",
					"weight": 1,
					"elements": []map[string]any{
						{
							"tag":        "markdown",
							"content":    "**当前模式**",
							"text_align": "left",
						},
					},
				},
				{
					"tag":    "column",
					"width":  "weighted",
					"weight": 3,
					"elements": []map[string]any{
						{
							"tag": "div",
							"text": map[string]any{
								"tag":     "plain_text",
								"content": modesInfo,
							},
						},
					},
				},
			},
		})
	}

	if models != nil && len(models.AvailableModels) > 0 {
		options := make([]map[string]any, len(models.AvailableModels))
		for i, m := range models.AvailableModels {
			options[i] = map[string]any{
				"text": map[string]any{
					"tag":     "plain_text",
					"content": m.Name,
				},
				"value": m.ModelId,
			}
		}
		elements = append(elements, map[string]any{
			"tag":                "column_set",
			"horizontal_spacing": "8px",
			"horizontal_align":   "left",
			"columns": []map[string]any{
				{
					"tag":    "column",
					"width":  "weighted",
					"weight": 1,
					"elements": []map[string]any{
						{
							"tag":        "markdown",
							"content":    "**模型**",
							"text_align": "left",
						},
					},
				},
				{
					"tag":    "column",
					"width":  "weighted",
					"weight": 3,
					"elements": []map[string]any{
						{
							"tag":            "select_static",
							"name":           "model_select",
							"initial_option": models.CurrentModelId,
							"options":        options,
							"width":          "fill",
							"behaviors": []map[string]any{
								{
									"type": "callback",
									"value": map[string]any{
										"action": "change_model",
									},
								},
							},
						},
					},
				},
			},
		})
	}
	if modes != nil && len(modes.AvailableModes) > 0 {
		options := make([]map[string]any, len(modes.AvailableModes))
		for i, m := range modes.AvailableModes {
			options[i] = map[string]any{
				"text": map[string]any{
					"tag":     "plain_text",
					"content": m.Name,
				},
				"value": m.Id,
			}
		}
		elements = append(elements, map[string]any{
			"tag":                "column_set",
			"horizontal_spacing": "8px",
			"horizontal_align":   "left",
			"columns": []map[string]any{
				{
					"tag":    "column",
					"width":  "weighted",
					"weight": 1,
					"elements": []map[string]any{
						{
							"tag":        "markdown",
							"content":    "**模式**",
							"text_align": "left",
						},
					},
				},
				{
					"tag":    "column",
					"width":  "weighted",
					"weight": 3,
					"elements": []map[string]any{
						{
							"tag":            "select_static",
							"name":           "mode_select",
							"initial_option": modes.CurrentModeId,
							"options":        options,
							"width":          "fill",
							"behaviors": []map[string]any{
								{
									"type": "callback",
									"value": map[string]any{
										"action": "change_mode",
									},
								},
							},
						},
					},
				},
			},
		})
	}

	card := map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": fmt.Sprintf("Agent: %s", agent),
			},
			"template": "blue",
		},
		"body": map[string]any{
			"elements": elements,
		},
	}
	data, _ := json.Marshal(card)
	return string(data)
}
