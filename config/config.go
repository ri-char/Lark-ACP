package config

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

type AgentConfig struct {
	Id  string            `toml:"id"`
	Cmd []string          `toml:"cmd"`
	Env map[string]string `toml:"env"`
}

type Config struct {
	FeishuAppID             string        `toml:"feishu_app_id"`
	FeishuAppSecret         string        `toml:"feishu_app_secret"`
	FeishuVerificationToken string        `toml:"feishu_verification_token"`
	FeishuEventEncryptKey   string        `toml:"feishu_event_encrypt_key"`
	Agents                  []AgentConfig `toml:"agent"`
}

func Load() (*Config, error) {
	configPath := getConfigPath()
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	decoder := toml.NewDecoder(file).DisallowUnknownFields()

	var output Config
	err = decoder.Decode(&output)
	if err != nil {
		return nil, err
	}

	return &output, nil
}

func getConfigPath() string {
	path, err := os.UserConfigDir()
	if err != nil {
		return "config.toml"
	}
	return filepath.Join(path, "lark-acp", "config.toml")
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
