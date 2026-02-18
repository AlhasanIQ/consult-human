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
