package config

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

type AgentConfig struct {
	Id  string   `toml:"id"`
	Cmd []string `toml:"cmd"`
}

type Config struct {
	FeishuAppID     string        `toml:"feishu_app_id"`
	FeishuAppSecret string        `toml:"feishu_app_secret"`
	Agents          []AgentConfig `toml:"agent"`
}

func Load() (*Config, error) {
	configPath := getConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var raw Config
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	return &raw, nil
}

func getConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "lark-acp", "config.toml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "lark-acp", "config.toml")
}

func (c *Config) FindAgentById(id string) (agentCfg *AgentConfig, exist bool) {
	for _, agent := range c.Agents {
		if agent.Id == id {
			agentCfg = &agent
			exist = true
			break
		}
	}
	return
}
