// Package config 负责读写客户端配置（cookie 等）。
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/ArkURL/nga-tui/internal/model"
)

// saveMu 保护配置文件的并发读写，避免多个请求同时持久化时写坏文件。
var saveMu sync.Mutex

// Config 是持久化配置。
type Config struct {
	UA        string            `json:"ua,omitempty"`
	Cookies   map[string]string `json:"cookies"`
	Favorites []model.BoardRef  `json:"favorites,omitempty"` // 收藏版面（兼容旧 fid 字符串）
}

// Dir 返回配置目录（~/.config/nga-tui），不存在则创建。
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "nga-tui")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load 读取配置；文件不存在时返回空配置。
func Load() (*Config, error) {
	saveMu.Lock()
	defer saveMu.Unlock()
	p, err := path()
	if err != nil {
		return nil, err
	}
	cfg := &Config{Cookies: map[string]string{}}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if cfg.Cookies == nil {
		cfg.Cookies = map[string]string{}
	}
	return cfg, nil
}

// Save 保存配置（0600 权限，仅本用户可读）。
func Save(cfg *Config) error {
	saveMu.Lock()
	defer saveMu.Unlock()
	p, err := path()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}
