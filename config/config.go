package config

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

type AgentConfig struct {
	Cmd string `toml:"cmd"`
}

type Config struct {
	FeishuAppID     string                 `toml:"feishu_app_id"`
	FeishuAppSecret string                 `toml:"feishu_app_secret"`
	DefaultAgent    string                 `toml:"default_agent"`
	Agents          map[string]AgentConfig `toml:"-"`
}

type rawConfig struct {
	FeishuAppID     string `toml:"feishu_app_id"`
	FeishuAppSecret string `toml:"feishu_app_secret"`
	DefaultAgent    string `toml:"default_agent"`
}

func Load() (*Config, error) {
	configPath := getConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var raw rawConfig
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	// Parse dynamic agent configs
	var rawMap map[string]interface{}
	if err := toml.Unmarshal(data, &rawMap); err != nil {
		return nil, err
	}

	agents := make(map[string]AgentConfig)
	for key, val := range rawMap {
		if key == "feishu_app_id" || key == "feishu_app_secret" || key == "feishu_user_id" || key == "default_agent" {
			continue
		}
		if m, ok := val.(map[string]interface{}); ok {
			agent := AgentConfig{}
			if cmd, ok := m["cmd"].(string); ok {
				agent.Cmd = cmd
			}
			agents[key] = agent
		}
	}

	return &Config{
		FeishuAppID:     raw.FeishuAppID,
		FeishuAppSecret: raw.FeishuAppSecret,
		DefaultAgent:    raw.DefaultAgent,
		Agents:         agents,
	}, nil
}

func getConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "lark-acp", "config.toml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "lark-acp", "config.toml")
}