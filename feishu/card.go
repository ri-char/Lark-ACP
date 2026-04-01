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
				"content": "选择" + agentId + "会话",
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

func LoadSessionAgentSessionFreezeCard(agentId string) any {
	// JSON 2.0 卡片结构
	cardV2 := map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": "加载会话",
			}, "subtitle": map[string]any{
				"tag":     "plain_text",
				"content": "选择" + agentId + "会话",
			},
			"template": "grey",
		},
		"body": map[string]any{
			"elements": []map[string]any{
				{
					"tag":  "form",
					"name": "path_form",
					"elements": []map[string]any{
						{
							"tag":      "select_static",
							"name":     "session_id",
							"disabled": true,
							"width":    "fill",
						},
						{
							"tag":  "button",
							"name": "load_session_session",
							"text": map[string]any{
								"tag":     "plain_text",
								"content": "提交",
							},
							"disabled":         true,
							"type":             "primary_filled",
							"form_action_type": "submit",
						},
					},
				},
			},
		},
	}

	return cardV2
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

func GroupPinHeaderCard(agent, path string, models *acpsdk.SessionModelState, modes *acpsdk.SessionModeState, title *string) string {
	var elements []map[string]any
	if title != nil {
		elements = append(elements,
			map[string]any{
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
								"content":    "**名称**",
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
								"content":    title,
								"text_align": "left",
							},
						},
					},
				},
			})
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
	})
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

func UsageHeaderCard(used, size int) string {
	var sizeText string
	if size >= 1000000 {
		sizeText = fmt.Sprintf("%dM", size / 1000000)
	} else if size >= 1000 {
		sizeText = fmt.Sprintf("%dK", size / 1000)
	} else {
		sizeText = fmt.Sprintf("%d", size)
	}
	text := fmt.Sprintf("上下文已用%.1f%%，共计%s", float32(used)/float32(size)*100.0, sizeText)
	card := map[string]any{
		"schema": "2.0",
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
	data, _ := json.Marshal(card)
	return string(data)
}
