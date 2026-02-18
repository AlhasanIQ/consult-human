package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	EnvConfigPath = "CONSULT_HUMAN_CONFIG"
)

type Config struct {
	ActiveProvider string         `yaml:"active_provider"`
	RequestTimeout string         `yaml:"request_timeout"`
	Telegram       TelegramConfig `yaml:"telegram"`
	WhatsApp       WhatsAppConfig `yaml:"whatsapp"`
}

type TelegramConfig struct {
	BotToken            string `yaml:"bot_token"`
	ChatID              int64  `yaml:"chat_id"`
	PollIntervalSeconds int    `yaml:"poll_interval_seconds"`
}

type WhatsAppConfig struct {
	Recipient string `yaml:"recipient"`
	StorePath string `yaml:"store_path"`
}

func Default() Config {
	return Config{
		ActiveProvider: "telegram",
		RequestTimeout: "15m",
		Telegram: TelegramConfig{
			PollIntervalSeconds: 2,
		},
		WhatsApp: WhatsAppConfig{},
	}
}

func ConfigPath() (string, error) {
	if p := strings.TrimSpace(os.Getenv(EnvConfigPath)); p != "" {
		return ExpandPath(p)
	}

	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "consult-human", "config.yaml"), nil
	}

	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "consult-human", "config.yaml"), nil
}

func DefaultWhatsAppStorePath() (string, error) {
	stateDir, err := DefaultStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(stateDir, "whatsapp.db"), nil
}

func DefaultStateDir() (string, error) {
	if xdg := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); xdg != "" {
		return filepath.Join(xdg, "consult-human"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "consult-human"), nil
}

func Load() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return Config{}, err
	}

	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg := Default()
			ApplyDefaults(&cfg)
			return cfg, nil
		}
		return Config{}, err
	}

	cfg := Default()
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	ApplyDefaults(&cfg)
	return cfg, nil
}

func Save(cfg Config) error {
	ApplyDefaults(&cfg)

	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	b, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, b, 0o600)
}

func ApplyDefaults(cfg *Config) {
	if cfg == nil {
		return
	}
	if strings.TrimSpace(cfg.ActiveProvider) == "" {
		cfg.ActiveProvider = "telegram"
	}
	cfg.ActiveProvider = strings.ToLower(strings.TrimSpace(cfg.ActiveProvider))
	// WhatsApp is temporarily disabled in this phase.
	if cfg.ActiveProvider == "whatsapp" {
		cfg.ActiveProvider = "telegram"
	}
	if strings.TrimSpace(cfg.RequestTimeout) == "" {
		cfg.RequestTimeout = "15m"
	}
	if cfg.Telegram.PollIntervalSeconds <= 0 {
		cfg.Telegram.PollIntervalSeconds = 2
	}
	storePath := strings.TrimSpace(cfg.WhatsApp.StorePath)
	if storePath == "" {
		if p, err := DefaultWhatsAppStorePath(); err == nil {
			cfg.WhatsApp.StorePath = p
		}
	} else if p, err := ExpandPath(storePath); err == nil {
		cfg.WhatsApp.StorePath = p
	} else {
		cfg.WhatsApp.StorePath = storePath
	}
}

func EffectiveTimeout(cfg Config) (time.Duration, error) {
	ApplyDefaults(&cfg)
	d, err := time.ParseDuration(cfg.RequestTimeout)
	if err != nil {
		return 0, fmt.Errorf("invalid request_timeout %q: %w", cfg.RequestTimeout, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("request_timeout must be > 0")
	}
	return d, nil
}

func Set(cfg *Config, key string, value string) error {
	if cfg == nil {
		return fmt.Errorf("nil config")
	}
	k := strings.ToLower(strings.TrimSpace(key))
	v := strings.TrimSpace(value)

	switch k {
	case "active_provider", "provider", "default-provider":
		v = strings.ToLower(v)
		if v == "whatsapp" {
			return fmt.Errorf("whatsapp is temporarily disabled")
		}
		if v != "telegram" {
			return fmt.Errorf("provider must be telegram")
		}
		cfg.ActiveProvider = v
	case "request_timeout":
		if _, err := time.ParseDuration(v); err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}
		cfg.RequestTimeout = v
	case "telegram.bot_token":
		cfg.Telegram.BotToken = v
	case "telegram.chat_id":
		chatID, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid telegram.chat_id: %w", err)
		}
		cfg.Telegram.ChatID = chatID
	case "telegram.poll_interval_seconds":
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return fmt.Errorf("telegram.poll_interval_seconds must be a positive integer")
		}
		cfg.Telegram.PollIntervalSeconds = n
	case "whatsapp.recipient":
		cfg.WhatsApp.Recipient = v
	case "whatsapp.store_path":
		expanded, err := ExpandPath(v)
		if err != nil {
			return err
		}
		cfg.WhatsApp.StorePath = expanded
	default:
		return fmt.Errorf("unsupported key %q", key)
	}

	ApplyDefaults(cfg)
	return nil
}

func Marshal(cfg Config) ([]byte, error) {
	ApplyDefaults(&cfg)
	return yaml.Marshal(cfg)
}

func ExpandPath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if raw == "~" || strings.HasPrefix(raw, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if raw == "~" {
			return home, nil
		}
		return filepath.Join(home, strings.TrimPrefix(raw, "~/")), nil
	}
	return raw, nil
}
