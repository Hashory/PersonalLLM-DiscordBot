package main

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type ApiChannelConfig struct {
	ApiUrl             string   `yaml:"api-url"`
	ApiAuthToken       string   `yaml:"api-auth-token"`
	ModelName          string   `yaml:"model-name"`
	SystemRoleMessages []string `yaml:"system-role-messages"`
	ChatChannelId      string   `yaml:"chat-channel-id"`
}

type Config struct {
	Token            string             `yaml:"token"`
	ApiChannelConfig []ApiChannelConfig `yaml:"api-channel-configs"`
}

// LoadConfig loads the configuration from the given file path.
func LoadConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Error reading config file: %v", err)
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Printf("Error unmarshal config: %v", err)
		return nil, err
	}

	return &config, nil
}

// FindChannelConfig returns the ApiChannelConfig for the given channel ID.
func (c *Config) FindChannelConfig(channelId string) *ApiChannelConfig {
	for _, apiConfig := range c.ApiChannelConfig {
		if apiConfig.ChatChannelId == channelId {
			return &apiConfig
		}
	}

	return nil
}
