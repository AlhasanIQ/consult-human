package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigPathFromEnv(t *testing.T) {
	t.Setenv(EnvConfigPath, "/tmp/custom-config.yaml")
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath returned error: %v", err)
	}
	if path != "/tmp/custom-config.yaml" {
		t.Fatalf("unexpected path: %s", path)
	}
}

func TestSetTelegramChatID(t *testing.T) {
	cfg := Default()
	if err := Set(&cfg, "telegram.chat_id", "12345"); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if cfg.Telegram.ChatID != 12345 {
		t.Fatalf("unexpected chat id: %d", cfg.Telegram.ChatID)
	}
}

func TestSetDefaultProviderAlias(t *testing.T) {
	cfg := Default()
	if err := Set(&cfg, "default-provider", "telegram"); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if cfg.ActiveProvider != "telegram" {
		t.Fatalf("unexpected active provider: %q", cfg.ActiveProvider)
	}
}

func TestSetDefaultProviderRejectsWhatsApp(t *testing.T) {
	cfg := Default()
	err := Set(&cfg, "default-provider", "whatsapp")
	if err == nil {
		t.Fatalf("expected error for disabled whatsapp provider")
	}
	if !strings.Contains(err.Error(), "temporarily disabled") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExpandPathHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home dir: %v", err)
	}
	got, err := ExpandPath("~/foo/bar")
	if err != nil {
		t.Fatalf("ExpandPath: %v", err)
	}
	want := filepath.Join(home, "foo", "bar")
	if got != want {
		t.Fatalf("want %q got %q", want, got)
	}
}

func TestApplyDefaultsExpandsWhatsAppStorePath(t *testing.T) {
	cfg := Default()
	cfg.WhatsApp.StorePath = "~/consult-human/test.db"

	ApplyDefaults(&cfg)

	if strings.HasPrefix(cfg.WhatsApp.StorePath, "~") {
		t.Fatalf("expected expanded path, got %q", cfg.WhatsApp.StorePath)
	}
}

func TestApplyDefaultsNormalizesDisabledWhatsAppProvider(t *testing.T) {
	cfg := Default()
	cfg.ActiveProvider = "whatsapp"

	ApplyDefaults(&cfg)

	if cfg.ActiveProvider != "telegram" {
		t.Fatalf("expected telegram active provider, got %q", cfg.ActiveProvider)
	}
}

func TestTelegramPendingStorePathFromEnv(t *testing.T) {
	t.Setenv(EnvTelegramPendingStorePath, "~/consult-human/pending.json")
	got, err := TelegramPendingStorePath()
	if err != nil {
		t.Fatalf("TelegramPendingStorePath: %v", err)
	}
	if strings.Contains(got, "~") {
		t.Fatalf("expected expanded path, got %q", got)
	}
}

func TestDefaultTelegramPendingStorePathUsesStateHome(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	got, err := DefaultTelegramPendingStorePath()
	if err != nil {
		t.Fatalf("DefaultTelegramPendingStorePath: %v", err)
	}
	want := filepath.Join(stateHome, "consult-human", "telegram-pending.json")
	if got != want {
		t.Fatalf("want %q got %q", want, got)
	}
}

func TestSetTelegramPendingStorePathAlias(t *testing.T) {
	cfg := Default()
	if err := Set(&cfg, "telegram.store_path", "~/consult-human/tg-pending.json"); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if strings.HasPrefix(cfg.Telegram.PendingStorePath, "~") {
		t.Fatalf("expected expanded path, got %q", cfg.Telegram.PendingStorePath)
	}
}

func TestEffectiveTelegramPendingStorePathUsesConfigWhenEnvMissing(t *testing.T) {
	cfg := Default()
	cfg.Telegram.PendingStorePath = "/tmp/custom-tg-pending.json"
	got, err := EffectiveTelegramPendingStorePath(cfg)
	if err != nil {
		t.Fatalf("EffectiveTelegramPendingStorePath: %v", err)
	}
	if got != "/tmp/custom-tg-pending.json" {
		t.Fatalf("want %q got %q", "/tmp/custom-tg-pending.json", got)
	}
}

func TestEffectiveTelegramPendingStorePathEnvOverridesConfig(t *testing.T) {
	t.Setenv(EnvTelegramPendingStorePath, "/tmp/env-tg-pending.json")
	cfg := Default()
	cfg.Telegram.PendingStorePath = "/tmp/config-tg-pending.json"
	got, err := EffectiveTelegramPendingStorePath(cfg)
	if err != nil {
		t.Fatalf("EffectiveTelegramPendingStorePath: %v", err)
	}
	if got != "/tmp/env-tg-pending.json" {
		t.Fatalf("want %q got %q", "/tmp/env-tg-pending.json", got)
	}
}
