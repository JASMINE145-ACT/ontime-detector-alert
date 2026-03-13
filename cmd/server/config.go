package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	PriceProvider struct {
		BaseURL string `yaml:"base_url"`
	} `yaml:"price_provider"`

	SchedulerIntervalSeconds int    `yaml:"scheduler_interval_seconds"`
	WeComWebhookURL          string `yaml:"wecom_webhook_url"`
	DatabasePath             string `yaml:"database_path"`
	ListenAddress            string `yaml:"listen_address"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
