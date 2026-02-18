package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlhasanIQ/consult-human/config"
)

func TestRunConfigResetDeletesFile(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, cfgPath)

	cfg := config.Default()
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := runConfig([]string{"reset"}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err != nil {
		t.Fatalf("runConfig reset returned error: %v", err)
	}
	if _, statErr := os.Stat(cfgPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected config file to be deleted, stat err: %v", statErr)
	}
	if !strings.Contains(errOut.String(), "Deleted config at") {
		t.Fatalf("expected delete message, got: %q", errOut.String())
	}
}

func TestRunConfigResetMissingFile(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "missing-config.yaml")
	t.Setenv(config.EnvConfigPath, cfgPath)

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := runConfig([]string{"reset"}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err != nil {
		t.Fatalf("runConfig reset returned error: %v", err)
	}
	if !strings.Contains(errOut.String(), "Config not found at") {
		t.Fatalf("expected not-found message, got: %q", errOut.String())
	}
}

func TestRunConfigResetSpecificProviderTelegram(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, cfgPath)

	cfg := config.Default()
	cfg.ActiveProvider = "telegram"
	cfg.Telegram.BotToken = "tg-token"
	cfg.Telegram.ChatID = 1234
	cfg.WhatsApp.Recipient = "+15551234567"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := runConfig([]string{"reset", "--provider", "telegram"}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err != nil {
		t.Fatalf("runConfig reset --provider telegram returned error: %v", err)
	}

	got, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if got.Telegram.BotToken != "" || got.Telegram.ChatID != 0 {
		t.Fatalf("expected telegram config to be cleared, got: %#v", got.Telegram)
	}
	if got.WhatsApp.Recipient == "" {
		t.Fatalf("expected whatsapp config to remain")
	}
	if got.ActiveProvider != "telegram" {
		t.Fatalf("expected active provider to remain telegram while whatsapp is disabled, got %q", got.ActiveProvider)
	}
	if !strings.Contains(errOut.String(), "Reset provider telegram") {
		t.Fatalf("expected reset message, got: %q", errOut.String())
	}
}

func TestRunConfigResetSpecificProviderInvalid(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, cfgPath)

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := runConfig([]string{"reset", "--provider", "sms"}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err == nil {
		t.Fatalf("expected error for invalid provider")
	}
	if !strings.Contains(err.Error(), "provider must be telegram or whatsapp") {
		t.Fatalf("unexpected error: %v", err)
	}
}
