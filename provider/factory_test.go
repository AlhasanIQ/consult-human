package provider

import (
	"strings"
	"testing"

	"github.com/AlhasanIQ/consult-human/config"
)

func TestFactoryRejectsDisabledWhatsApp(t *testing.T) {
	cfg := config.Default()
	cfg.Telegram.BotToken = "test-token"

	_, err := New(cfg, "whatsapp")
	if err == nil {
		t.Fatalf("expected error for disabled whatsapp provider")
	}
	if !strings.Contains(err.Error(), "temporarily disabled") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFactoryUsesTelegram(t *testing.T) {
	cfg := config.Default()
	cfg.ActiveProvider = "telegram"
	cfg.Telegram.BotToken = "test-token"

	p, err := New(cfg, "")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if p.Name() != "telegram" {
		t.Fatalf("expected telegram provider, got %q", p.Name())
	}
	_ = p.Close()
}
