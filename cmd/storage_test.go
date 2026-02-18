package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlhasanIQ/consult-human/config"
)

func TestRunStorageClearTelegram(t *testing.T) {
	setTestStateHome(t)

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, cfgPath)

	tgPath := filepath.Join(t.TempDir(), "telegram-pending.json")
	t.Setenv(config.EnvTelegramPendingStorePath, tgPath)
	if err := os.WriteFile(tgPath, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write telegram pending: %v", err)
	}
	if err := os.WriteFile(tgPath+".lock", []byte("1\n"), 0o600); err != nil {
		t.Fatalf("write telegram lock: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := runStorage([]string{"clear", "--provider", "telegram"}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err != nil {
		t.Fatalf("runStorage clear telegram: %v", err)
	}
	if _, statErr := os.Stat(tgPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected telegram pending store removed, stat err: %v", statErr)
	}
	if _, statErr := os.Stat(tgPath + ".lock"); !os.IsNotExist(statErr) {
		t.Fatalf("expected telegram pending lock removed, stat err: %v", statErr)
	}
	if !strings.Contains(errOut.String(), "Cleared storage for telegram") {
		t.Fatalf("expected clear summary, got: %q", errOut.String())
	}
}

func TestRunStorageClearWhatsApp(t *testing.T) {
	setTestStateHome(t)

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, cfgPath)

	dbPath := filepath.Join(t.TempDir(), "wa.db")
	cfg := config.Default()
	cfg.WhatsApp.StorePath = dbPath
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	for _, p := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := runStorage([]string{"clear", "--provider", "whatsapp"}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err != nil {
		t.Fatalf("runStorage clear whatsapp: %v", err)
	}
	for _, p := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if _, statErr := os.Stat(p); !os.IsNotExist(statErr) {
			t.Fatalf("expected %s removed, stat err: %v", p, statErr)
		}
	}
	if !strings.Contains(errOut.String(), "Cleared storage for whatsapp") {
		t.Fatalf("expected clear summary, got: %q", errOut.String())
	}
}

func TestRunStorageClearInvalidProvider(t *testing.T) {
	setTestStateHome(t)

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, cfgPath)

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := runStorage([]string{"clear", "--provider", "sms"}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err == nil {
		t.Fatalf("expected invalid provider error")
	}
	if !strings.Contains(err.Error(), "provider must be all, telegram, or whatsapp") {
		t.Fatalf("unexpected error: %v", err)
	}
}
