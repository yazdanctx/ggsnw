package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	configDirName  = ".config/ggsnw"
	configFileName = "config.json"
)

type Config struct {
	GitHubToken string `json:"github_token,omitempty"`
	OpenAIKey   string `json:"openai_key,omitempty"`
}

func Path() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, configDirName, configFileName)
}

func Load() (*Config, error) {
	data, err := os.ReadFile(Path())
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func Save(update func(*Config)) error {
	cfg := &Config{}
	if data, err := os.ReadFile(Path()); err == nil {
		json.Unmarshal(data, cfg)
	}
	update(cfg)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}
