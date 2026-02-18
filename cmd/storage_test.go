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
	inboxPath := filepath.Join(filepath.Dir(tgPath), "telegram-inbox.json")
	pollerLock := filepath.Join(filepath.Dir(inboxPath), "telegram-poller.lock")
	if err := os.WriteFile(tgPath, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write telegram pending: %v", err)
	}
	if err := os.WriteFile(tgPath+".lock", []byte("1\n"), 0o600); err != nil {
		t.Fatalf("write telegram lock: %v", err)
	}
	if err := os.WriteFile(inboxPath, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write telegram inbox: %v", err)
	}
	if err := os.WriteFile(pollerLock, []byte("1\n"), 0o600); err != nil {
		t.Fatalf("write telegram poller lock: %v", err)
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
	if _, statErr := os.Stat(inboxPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected telegram inbox removed, stat err: %v", statErr)
	}
	if _, statErr := os.Stat(pollerLock); !os.IsNotExist(statErr) {
		t.Fatalf("expected telegram poller lock removed, stat err: %v", statErr)
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

func TestRunStoragePathTelegramUsesConfigOverride(t *testing.T) {
	setTestStateHome(t)

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, cfgPath)

	cfg := config.Default()
	cfg.Telegram.PendingStorePath = filepath.Join(t.TempDir(), "custom-telegram-pending.json")
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	expectedInbox, err := config.EffectiveTelegramInboxStorePath(cfg)
	if err != nil {
		t.Fatalf("EffectiveTelegramInboxStorePath: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err = runStorage([]string{"path", "--provider", "telegram"}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err != nil {
		t.Fatalf("runStorage path telegram: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "pending: "+cfg.Telegram.PendingStorePath) {
		t.Fatalf("missing pending path, got: %q", got)
	}
	if !strings.Contains(got, "inbox: "+expectedInbox) {
		t.Fatalf("missing inbox path, got: %q", got)
	}
}

func TestRunStoragePathAll(t *testing.T) {
	setTestStateHome(t)

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, cfgPath)

	cfg := config.Default()
	cfg.Telegram.PendingStorePath = filepath.Join(t.TempDir(), "custom-telegram-pending.json")
	cfg.WhatsApp.StorePath = filepath.Join(t.TempDir(), "custom-whatsapp.db")
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := runStorage([]string{"path"}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err != nil {
		t.Fatalf("runStorage path all: %v", err)
	}
	got := out.String()
	expectedInbox, err := config.EffectiveTelegramInboxStorePath(cfg)
	if err != nil {
		t.Fatalf("EffectiveTelegramInboxStorePath: %v", err)
	}
	skillManagedPath, err := defaultManagedSkillSourcePath()
	if err != nil {
		t.Fatalf("defaultManagedSkillSourcePath: %v", err)
	}
	if !strings.Contains(got, "telegram.pending: "+cfg.Telegram.PendingStorePath) {
		t.Fatalf("missing telegram path, got: %q", got)
	}
	if !strings.Contains(got, "telegram.inbox: "+expectedInbox) {
		t.Fatalf("missing telegram inbox path, got: %q", got)
	}
	if !strings.Contains(got, "whatsapp: "+cfg.WhatsApp.StorePath) {
		t.Fatalf("missing whatsapp path, got: %q", got)
	}
	if !strings.Contains(got, "skill.managed: "+skillManagedPath) {
		t.Fatalf("missing managed skill path, got: %q", got)
	}
}

func TestRunStorageClearAllRemovesManagedSkill(t *testing.T) {
	setTestStateHome(t)

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, cfgPath)

	cfg := config.Default()
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	managedSkillPath, err := defaultManagedSkillSourcePath()
	if err != nil {
		t.Fatalf("defaultManagedSkillSourcePath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(managedSkillPath), 0o755); err != nil {
		t.Fatalf("mkdir managed skill dir: %v", err)
	}
	if err := os.WriteFile(managedSkillPath, []byte("skill"), 0o600); err != nil {
		t.Fatalf("write managed skill: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err = runStorage([]string{"clear"}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err != nil {
		t.Fatalf("runStorage clear all: %v", err)
	}

	if _, statErr := os.Stat(managedSkillPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected managed skill file removed, stat err: %v", statErr)
	}
}
