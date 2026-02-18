package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AlhasanIQ/consult-human/config"
)

func TestRunSkillInstallClaudeCopy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "SKILL.md")
	sourceContent := "name: consult-human\n\nsample"
	if err := os.WriteFile(sourcePath, []byte(sourceContent), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := runSkill([]string{"install", "--target", "claude", "--source", sourcePath, "--copy"}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err != nil {
		t.Fatalf("runSkill install returned error: %v", err)
	}

	installedPath := filepath.Join(home, ".claude", "skills", "consult-human", "SKILL.md")
	b, readErr := os.ReadFile(installedPath)
	if readErr != nil {
		t.Fatalf("read installed skill: %v", readErr)
	}
	if string(b) != sourceContent {
		t.Fatalf("unexpected installed content: %q", string(b))
	}

	codexPath := filepath.Join(home, ".codex", "skills", "consult-human", "SKILL.md")
	if _, statErr := os.Stat(codexPath); !os.IsNotExist(statErr) {
		t.Fatalf("did not expect codex install path to exist, stat err: %v", statErr)
	}
}

func TestRunSkillInstallCodexCreatesPathWhenExplicit(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "SKILL.md")
	sourceContent := "codex-skill"
	if err := os.WriteFile(sourcePath, []byte(sourceContent), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := runSkill([]string{"install", "--target", "codex", "--source", sourcePath, "--copy"}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err != nil {
		t.Fatalf("runSkill install returned error: %v", err)
	}

	installedPath := filepath.Join(home, ".codex", "skills", "consult-human", "SKILL.md")
	b, readErr := os.ReadFile(installedPath)
	if readErr != nil {
		t.Fatalf("read installed codex skill: %v", readErr)
	}
	if string(b) != sourceContent {
		t.Fatalf("unexpected installed content: %q", string(b))
	}
}

func TestRunSkillInstallDefaultBothCreatesCodexWhenMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "SKILL.md")
	if err := os.WriteFile(sourcePath, []byte("sample"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := runSkill([]string{"install", "--source", sourcePath}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err != nil {
		t.Fatalf("runSkill install returned error: %v", err)
	}

	claudePath := filepath.Join(home, ".claude", "skills", "consult-human", "SKILL.md")
	if _, statErr := os.Stat(claudePath); statErr != nil {
		t.Fatalf("expected claude install path, stat err: %v", statErr)
	}

	codexPath := filepath.Join(home, ".codex", "skills", "consult-human", "SKILL.md")
	if _, statErr := os.Stat(codexPath); statErr != nil {
		t.Fatalf("expected codex path to be created for default both target, stat err: %v", statErr)
	}
}

func TestRunSkillInstallClaudeDoesNotEmitAgentsSkipNote(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "SKILL.md")
	if err := os.WriteFile(sourcePath, []byte("sample"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := runSkill([]string{"install", "--target", "claude", "--source", sourcePath}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err != nil {
		t.Fatalf("runSkill install returned error: %v", err)
	}
	if strings.Contains(errOut.String(), "Skipped ~/.agents/skills/consult-human") {
		t.Fatalf("did not expect agents skip note, got: %q", errOut.String())
	}
}

func TestRunSkillInstallLinkMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior differs on windows permissions")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)

	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "SKILL.md")
	if err := os.WriteFile(sourcePath, []byte("symlink-source"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := runSkill([]string{"install", "--target", "claude", "--source", sourcePath, "--link"}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err != nil {
		t.Fatalf("runSkill install --link returned error: %v", err)
	}

	installedPath := filepath.Join(home, ".claude", "skills", "consult-human", "SKILL.md")
	info, statErr := os.Lstat(installedPath)
	if statErr != nil {
		t.Fatalf("lstat installed path: %v", statErr)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected symlink at %s", installedPath)
	}
}

func TestRunSkillInstallRepoScoped(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repoDir := t.TempDir()
	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "SKILL.md")
	sourceContent := "repo-scoped-skill"
	if err := os.WriteFile(sourcePath, []byte(sourceContent), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := runSkill([]string{"install", "--target", "codex", "--repo", repoDir, "--source", sourcePath}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err != nil {
		t.Fatalf("runSkill install --repo returned error: %v", err)
	}

	repoInstallPath := filepath.Join(repoDir, ".codex", "skills", "consult-human", "SKILL.md")
	b, readErr := os.ReadFile(repoInstallPath)
	if readErr != nil {
		t.Fatalf("read repo install path: %v", readErr)
	}
	if string(b) != sourceContent {
		t.Fatalf("unexpected repo-installed content: %q", string(b))
	}

	globalInstallPath := filepath.Join(home, ".codex", "skills", "consult-human", "SKILL.md")
	if _, statErr := os.Stat(globalInstallPath); !os.IsNotExist(statErr) {
		t.Fatalf("did not expect global codex install when --repo is used, stat err: %v", statErr)
	}
}

func TestRunSkillInstallDefaultSourcePathInConfigDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior differs on windows permissions")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgPath := filepath.Join(t.TempDir(), "consult-human", "config.yaml")
	t.Setenv(config.EnvConfigPath, cfgPath)

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := runSkill([]string{"install", "--target", "claude"}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err != nil {
		t.Fatalf("default-source runSkill install returned error: %v", err)
	}

	managedSourcePath := filepath.Join(filepath.Dir(cfgPath), "SKILL.md")
	managedSourceBytes, readErr := os.ReadFile(managedSourcePath)
	if readErr != nil {
		t.Fatalf("read managed source path: %v", readErr)
	}
	if len(bytes.TrimSpace(managedSourceBytes)) == 0 {
		t.Fatalf("expected non-empty managed source")
	}
	if !bytes.Equal(bytes.TrimSpace(managedSourceBytes), bytes.TrimSpace(skillTemplateEmbedded)) {
		t.Fatalf("expected managed source to match embedded skill template")
	}

	installedPath := filepath.Join(home, ".claude", "skills", "consult-human", "SKILL.md")
	info, statErr := os.Lstat(installedPath)
	if statErr != nil {
		t.Fatalf("lstat installed skill path: %v", statErr)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected default install to create symlink")
	}
	linkTarget, linkErr := os.Readlink(installedPath)
	if linkErr != nil {
		t.Fatalf("readlink installed skill path: %v", linkErr)
	}
	if filepath.Clean(linkTarget) != filepath.Clean(managedSourcePath) {
		t.Fatalf("expected symlink target %s, got %s", managedSourcePath, linkTarget)
	}
}

func TestEmbeddedSkillTemplateMatchesRootSkillDoc(t *testing.T) {
	repoSkillPath := filepath.Join("..", "SKILL.md")
	repoSkillBytes, err := os.ReadFile(repoSkillPath)
	if err != nil {
		t.Fatalf("read repo SKILL.md: %v", err)
	}

	if !bytes.Equal(bytes.TrimSpace(repoSkillBytes), bytes.TrimSpace(skillTemplateEmbedded)) {
		t.Fatalf("cmd/skill_template.md is out of sync with SKILL.md; update cmd/skill_template.md")
	}
}

func TestNormalizeSkillTargetAliases(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "claude", want: skillTargetClaude},
		{in: "cloud", want: skillTargetClaude},
		{in: "cloud-code", want: skillTargetClaude},
		{in: "codex", want: skillTargetCodex},
		{in: "both", want: skillTargetBoth},
		{in: "all", want: skillTargetBoth},
	}
	for _, tc := range tests {
		got, err := normalizeSkillTarget(tc.in)
		if err != nil {
			t.Fatalf("normalizeSkillTarget(%q) returned error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("normalizeSkillTarget(%q): want %q got %q", tc.in, tc.want, got)
		}
	}
}

func TestRunSkillInstallWritesClaudeReminderFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "SKILL.md")
	if err := os.WriteFile(sourcePath, []byte("sample"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := runSkill([]string{"install", "--target", "claude", "--source", sourcePath}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err != nil {
		t.Fatalf("runSkill install returned error: %v", err)
	}

	claudePath := filepath.Join(home, ".claude", "CLAUDE.md")
	b, readErr := os.ReadFile(claudePath)
	if readErr != nil {
		t.Fatalf("read claude reminder file: %v", readErr)
	}
	content := string(b)
	if !strings.Contains(content, consultHumanReminderStart) || !strings.Contains(content, consultHumanReminderEnd) {
		t.Fatalf("expected reminder block markers in CLAUDE.md, got: %q", content)
	}
	if !strings.Contains(content, "Never forget") {
		t.Fatalf("expected reminder content in CLAUDE.md, got: %q", content)
	}
}

func TestRunSkillInstallWritesCodexReminderFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "SKILL.md")
	if err := os.WriteFile(sourcePath, []byte("sample"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := runSkill([]string{"install", "--target", "codex", "--source", sourcePath}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err != nil {
		t.Fatalf("runSkill install returned error: %v", err)
	}

	codexPath := filepath.Join(home, ".codex", "AGENTS.md")
	b, readErr := os.ReadFile(codexPath)
	if readErr != nil {
		t.Fatalf("read codex reminder file: %v", readErr)
	}
	content := string(b)
	if !strings.Contains(content, consultHumanReminderStart) || !strings.Contains(content, consultHumanReminderEnd) {
		t.Fatalf("expected reminder block markers in AGENTS.md, got: %q", content)
	}
	if !strings.Contains(content, "consult-human ask") {
		t.Fatalf("expected consult-human reminder content in AGENTS.md, got: %q", content)
	}
}

func TestRunSkillInstallReminderIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "SKILL.md")
	if err := os.WriteFile(sourcePath, []byte("sample"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	for i := 0; i < 2; i++ {
		err := runSkill([]string{"install", "--target", "codex", "--source", sourcePath}, IO{
			In:     strings.NewReader(""),
			Out:    &out,
			ErrOut: &errOut,
		})
		if err != nil {
			t.Fatalf("runSkill install returned error on iteration %d: %v", i, err)
		}
	}

	codexPath := filepath.Join(home, ".codex", "AGENTS.md")
	b, readErr := os.ReadFile(codexPath)
	if readErr != nil {
		t.Fatalf("read codex reminder file: %v", readErr)
	}
	content := string(b)
	if count := strings.Count(content, consultHumanReminderStart); count != 1 {
		t.Fatalf("expected one reminder block, found %d in content: %q", count, content)
	}
}

func TestRunSkillInstallWritesAgentsReminderWhenAgentsDirExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".agents"), 0o755); err != nil {
		t.Fatalf("mkdir .agents: %v", err)
	}

	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "SKILL.md")
	if err := os.WriteFile(sourcePath, []byte("sample"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := runSkill([]string{"install", "--target", "claude", "--source", sourcePath}, IO{
		In:     strings.NewReader(""),
		Out:    &out,
		ErrOut: &errOut,
	})
	if err != nil {
		t.Fatalf("runSkill install returned error: %v", err)
	}

	agentsPath := filepath.Join(home, ".agents", "AGENTS.md")
	b, readErr := os.ReadFile(agentsPath)
	if readErr != nil {
		t.Fatalf("read agents reminder file: %v", readErr)
	}
	content := string(b)
	if !strings.Contains(content, consultHumanReminderStart) || !strings.Contains(content, consultHumanReminderEnd) {
		t.Fatalf("expected reminder block markers in agents AGENTS.md, got: %q", content)
	}
}
