package cmd

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/AlhasanIQ/consult-human/config"
)

func stubSetupEnsureShellPath(t *testing.T) {
	t.Helper()
	origEnsurePathFn := setupEnsureShellPathFn
	setupEnsureShellPathFn = func() (setupShellPathStatus, error) {
		return setupShellPathStatus{
			Shell:          "zsh",
			ProfilePath:    "/tmp/.zshenv",
			BinaryDir:      "/tmp/bin",
			AlreadyPresent: true,
		}, nil
	}
	t.Cleanup(func() { setupEnsureShellPathFn = origEnsurePathFn })
}

func TestParseSetupProviderFlags(t *testing.T) {
	got, err := parseSetupProviderFlags([]string{"telegram"})
	if err != nil {
		t.Fatalf("parseSetupProviderFlags returned error: %v", err)
	}
	want := []string{setupProviderTelegram}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %#v got %#v", want, got)
	}
}

func TestParseSetupSkillTargetSelection(t *testing.T) {
	got, err := parseSetupSkillTargetSelection("1,2")
	if err != nil {
		t.Fatalf("parseSetupSkillTargetSelection returned error: %v", err)
	}
	if got != skillTargetBoth {
		t.Fatalf("expected %q, got %q", skillTargetBoth, got)
	}
}

func TestParseSetupSkillTargetSelectionInvalid(t *testing.T) {
	if _, err := parseSetupSkillTargetSelection("x"); err == nil {
		t.Fatalf("expected error for invalid selection")
	}
}

func TestBuildSetupSkillScopeOptionsDefaultsToRepoWhenAgentDirExists(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}

	origCurrentDirFn := setupCurrentDirFn
	setupCurrentDirFn = func() (string, error) { return repo, nil }
	defer func() { setupCurrentDirFn = origCurrentDirFn }()

	opts, defaultToken, err := buildSetupSkillScopeOptions(skillTargetClaude)
	if err != nil {
		t.Fatalf("buildSetupSkillScopeOptions: %v", err)
	}
	if len(opts) < 2 {
		t.Fatalf("expected at least global and repo options")
	}
	if defaultToken != "repo" {
		t.Fatalf("expected repo default, got %q", defaultToken)
	}
}

func TestFindGitRepoRoot(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	sub := filepath.Join(repo, "a", "b")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}

	got, found, err := findGitRepoRoot(sub)
	if err != nil {
		t.Fatalf("findGitRepoRoot: %v", err)
	}
	if !found {
		t.Fatalf("expected repo to be found")
	}
	if got != repo {
		t.Fatalf("expected repo %q got %q", repo, got)
	}
}

func TestParseSetupProviderFlagsRejectsInvalid(t *testing.T) {
	if _, err := parseSetupProviderFlags([]string{"3"}); err == nil {
		t.Fatalf("expected error for invalid option")
	}
}

func TestParseSetupProviderFlagsRejectsWhatsApp(t *testing.T) {
	if _, err := parseSetupProviderFlags([]string{"whatsapp"}); err == nil {
		t.Fatalf("expected error for disabled whatsapp")
	}
}

func TestRunSetupTelegramFlow(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, cfgPath)
	stubSetupEnsureShellPath(t)

	origSkillFn := setupSkillInstallFn
	defer func() { setupSkillInstallFn = origSkillFn }()
	var installedTarget string
	setupSkillInstallFn = func(args []string, io IO) error {
		if len(args) != 2 || args[0] != "--target" {
			return fmt.Errorf("unexpected skill args: %#v", args)
		}
		installedTarget = args[1]
		return nil
	}

	origLinkFn := telegramSetupLinkFn
	telegramSetupLinkFn = func(token string, timeout time.Duration, w io.Writer) (int64, error) {
		if token != "test-token" {
			return 0, fmt.Errorf("unexpected token: %s", token)
		}
		return 4242, nil
	}
	defer func() { telegramSetupLinkFn = origLinkFn }()

	origCurrentDirFn := setupCurrentDirFn
	setupCurrentDirFn = func() (string, error) { return t.TempDir(), nil }
	defer func() { setupCurrentDirFn = origCurrentDirFn }()

	input := strings.NewReader("test-token\n1\n1\n")
	var out bytes.Buffer
	var errOut bytes.Buffer

	if err := runSetup(nil, IO{In: input, Out: &out, ErrOut: &errOut}); err != nil {
		t.Fatalf("runSetup returned error: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load returned error: %v", err)
	}
	if got, want := cfg.Telegram.BotToken, "test-token"; got != want {
		t.Fatalf("want telegram token %q got %q", want, got)
	}
	if got, want := cfg.ActiveProvider, setupProviderTelegram; got != want {
		t.Fatalf("want active provider %q got %q", want, got)
	}
	if got, want := cfg.Telegram.ChatID, int64(4242); got != want {
		t.Fatalf("want telegram chat id %d got %d", want, got)
	}
	if installedTarget != skillTargetClaude {
		t.Fatalf("expected skill target %q, got %q", skillTargetClaude, installedTarget)
	}
	if strings.Contains(errOut.String(), "Next steps:") {
		t.Fatalf("did not expect Next steps block in output, got: %q", errOut.String())
	}
}

func TestRunSetupRejectsAlreadyConfiguredProvider(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, cfgPath)
	stubSetupEnsureShellPath(t)

	cfg := config.Default()
	cfg.Telegram.BotToken = "existing-token"
	cfg.Telegram.ChatID = 999
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save returned error: %v", err)
	}

	input := strings.NewReader("")
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := runSetup(nil, IO{In: input, Out: &out, ErrOut: &errOut})
	if err == nil {
		t.Fatalf("expected error for already configured provider")
	}
	if !strings.Contains(err.Error(), "config reset --provider telegram") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSetupTelegramUsesSavedTokenWithoutPrompt(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, cfgPath)
	stubSetupEnsureShellPath(t)

	origCurrentDirFn := setupCurrentDirFn
	setupCurrentDirFn = func() (string, error) { return t.TempDir(), nil }
	defer func() { setupCurrentDirFn = origCurrentDirFn }()

	origSkillFn := setupSkillInstallFn
	defer func() { setupSkillInstallFn = origSkillFn }()
	setupSkillInstallFn = func(args []string, io IO) error {
		if len(args) != 2 || args[0] != "--target" || args[1] != skillTargetClaude {
			return fmt.Errorf("unexpected skill args: %#v", args)
		}
		return nil
	}

	cfg := config.Default()
	cfg.Telegram.BotToken = "saved-token"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save returned error: %v", err)
	}

	origLinkFn := telegramSetupLinkFn
	telegramSetupLinkFn = func(token string, timeout time.Duration, w io.Writer) (int64, error) {
		if token != "saved-token" {
			return 0, fmt.Errorf("unexpected token: %s", token)
		}
		return 777, nil
	}
	defer func() { telegramSetupLinkFn = origLinkFn }()

	var out bytes.Buffer
	var errOut bytes.Buffer

	done := make(chan error, 1)
	go func() {
		done <- runSetup([]string{"--provider", "telegram"}, IO{
			In:     strings.NewReader("1\n1\n"),
			Out:    &out,
			ErrOut: &errOut,
		})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runSetup returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("runSetup appeared to block waiting for token prompt")
	}

	updated, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load returned error: %v", err)
	}
	if got, want := updated.Telegram.BotToken, "saved-token"; got != want {
		t.Fatalf("want telegram token %q got %q", want, got)
	}
	if got, want := updated.Telegram.ChatID, int64(777); got != want {
		t.Fatalf("want telegram chat id %d got %d", want, got)
	}
	if strings.Contains(errOut.String(), "Bot token:") {
		t.Fatalf("did not expect token prompt when token already exists, got: %q", errOut.String())
	}
}

func TestRunSetupRejectsWhatsAppProvider(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, cfgPath)

	input := strings.NewReader("")
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := runSetup([]string{"--provider", "whatsapp"}, IO{In: input, Out: &out, ErrOut: &errOut})
	if err == nil {
		t.Fatalf("expected error for disabled whatsapp provider")
	}
	if !strings.Contains(err.Error(), "temporarily disabled") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSetupNonInteractiveChecklistTelegram(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, cfgPath)
	stubSetupEnsureShellPath(t)

	var out bytes.Buffer
	var errOut bytes.Buffer

	if err := runSetup([]string{"--non-interactive", "--provider", "telegram"}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	}); err != nil {
		t.Fatalf("runSetup returned error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Setup checklist (non-interactive)") {
		t.Fatalf("expected checklist header, got: %q", got)
	}
	if !strings.Contains(got, "Shell PATH:") {
		t.Fatalf("expected shell PATH status block, got: %q", got)
	}
	if !strings.Contains(got, "consult-human config set telegram.bot_token") {
		t.Fatalf("expected telegram token command, got: %q", got)
	}
	if !strings.Contains(got, "consult-human setup --provider telegram --link-chat") {
		t.Fatalf("expected telegram non-interactive link command, got: %q", got)
	}
	if !strings.Contains(got, "consult-human config set default-provider telegram") {
		t.Fatalf("expected default-provider command, got: %q", got)
	}
	if !strings.Contains(got, "consult-human skill install --target claude") {
		t.Fatalf("expected skill install command, got: %q", got)
	}
	if !strings.Contains(got, "Final step: install skill files for agent runtimes, and a reminder in CLAUDE.md/AGENTS.md (required):") {
		t.Fatalf("expected required skill-install final step, got: %q", got)
	}
	if !strings.Contains(got, "Ask your human whether to install globally or locally (repo-only).") {
		t.Fatalf("expected ask-human scope guidance, got: %q", got)
	}
	if !strings.Contains(got, "Global install (all repos on this machine):") {
		t.Fatalf("expected global install heading, got: %q", got)
	}
	if !strings.Contains(got, "Local install (repo-only):") {
		t.Fatalf("expected local install heading, got: %q", got)
	}
}

func TestRunSetupLinkChatWithSavedToken(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, cfgPath)
	stubSetupEnsureShellPath(t)

	cfg := config.Default()
	cfg.Telegram.BotToken = "saved-token"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save returned error: %v", err)
	}

	origLinkFn := telegramSetupLinkFn
	telegramSetupLinkFn = func(token string, timeout time.Duration, w io.Writer) (int64, error) {
		if token != "saved-token" {
			return 0, fmt.Errorf("unexpected token: %s", token)
		}
		return 999, nil
	}
	defer func() { telegramSetupLinkFn = origLinkFn }()

	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := runSetup([]string{"--provider", "telegram", "--link-chat"}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	}); err != nil {
		t.Fatalf("runSetup returned error: %v", err)
	}

	updated, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load returned error: %v", err)
	}
	if got, want := updated.Telegram.ChatID, int64(999); got != want {
		t.Fatalf("want telegram chat id %d got %d", want, got)
	}
}

func TestRunSetupLinkChatRequiresToken(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, cfgPath)
	stubSetupEnsureShellPath(t)

	if err := config.Save(config.Default()); err != nil {
		t.Fatalf("config.Save returned error: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := runSetup([]string{"--provider", "telegram", "--link-chat"}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err == nil {
		t.Fatalf("expected error when token is missing")
	}
	if !strings.Contains(err.Error(), "telegram.bot_token is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSetupNonInteractiveEnsuresShellPath(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, cfgPath)

	origEnsurePathFn := setupEnsureShellPathFn
	calls := 0
	setupEnsureShellPathFn = func() (setupShellPathStatus, error) {
		calls++
		return setupShellPathStatus{
			Shell:          "zsh",
			ProfilePath:    "/tmp/.zshenv",
			BinaryDir:      "/tmp/bin",
			AlreadyPresent: true,
		}, nil
	}
	defer func() { setupEnsureShellPathFn = origEnsurePathFn }()

	var out bytes.Buffer
	var errOut bytes.Buffer

	if err := runSetup([]string{"--non-interactive"}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	}); err != nil {
		t.Fatalf("runSetup returned error: %v", err)
	}

	if calls != 1 {
		t.Fatalf("expected shell-path ensure to be called once, got %d", calls)
	}
}

func TestRunSetupNonInteractiveRejectsWhatsAppProvider(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, cfgPath)
	stubSetupEnsureShellPath(t)

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := runSetup([]string{"--non-interactive", "--provider", "whatsapp"}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err == nil {
		t.Fatalf("expected error for disabled whatsapp provider")
	}
	if !strings.Contains(err.Error(), "temporarily disabled") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSetupNonInteractiveShowsConfiguredProviderStatus(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, cfgPath)
	stubSetupEnsureShellPath(t)

	cfg := config.Default()
	cfg.Telegram.BotToken = "existing-token"
	cfg.Telegram.ChatID = 111
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save returned error: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer

	if err := runSetup([]string{"--non-interactive"}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	}); err != nil {
		t.Fatalf("runSetup returned error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Telegram (already set up):") {
		t.Fatalf("expected configured telegram status, got: %q", got)
	}
	if !strings.Contains(got, "config reset --provider telegram") {
		t.Fatalf("expected telegram reset guidance, got: %q", got)
	}
	if !strings.Contains(got, "WhatsApp (temporarily disabled):") {
		t.Fatalf("expected whatsapp deferred note, got: %q", got)
	}
}

func TestRunSetupNonInteractiveRejectsConfiguredProvider(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(config.EnvConfigPath, cfgPath)
	stubSetupEnsureShellPath(t)

	cfg := config.Default()
	cfg.Telegram.BotToken = "existing-token"
	cfg.Telegram.ChatID = 111
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save returned error: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := runSetup([]string{"--non-interactive", "--provider", "telegram"}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err == nil {
		t.Fatalf("expected error for already configured provider")
	}
	if !strings.Contains(err.Error(), "config reset --provider telegram") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitForTelegramStartWithBaseURL(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getUpdates" {
			http.NotFound(w, r)
			return
		}
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			_, _ = io.WriteString(w, `{"ok":true,"result":[{"update_id":1,"message":{"text":"/help","chat":{"id":123}}}]}`)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true,"result":[{"update_id":2,"message":{"text":"/start","chat":{"id":456}}}]}`)
	}))
	defer srv.Close()

	var out bytes.Buffer
	chatID, err := waitForTelegramStartWithBaseURL(srv.URL, 2*time.Second, &out)
	if err != nil {
		t.Fatalf("waitForTelegramStartWithBaseURL returned error: %v", err)
	}
	if chatID != 456 {
		t.Fatalf("expected chatID 456, got %d", chatID)
	}
}

func TestWaitForTelegramStartWithBaseURLTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true,"result":[]}`)
	}))
	defer srv.Close()

	var out bytes.Buffer
	_, err := waitForTelegramStartWithBaseURL(srv.URL, 120*time.Millisecond, &out)
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out waiting for /start") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitForTelegramStartWithBaseURLWebhookActive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"ok":false,"description":"Conflict: can't use getUpdates method while webhook is active"}`)
	}))
	defer srv.Close()

	var out bytes.Buffer
	_, err := waitForTelegramStartWithBaseURL(srv.URL, time.Second, &out)
	if err == nil {
		t.Fatalf("expected webhook-active error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "webhook") {
		t.Fatalf("expected webhook guidance error, got: %v", err)
	}
}
