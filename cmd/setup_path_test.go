package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveSetupProfilePathZshPrefersExisting(t *testing.T) {
	home := t.TempDir()
	zprofile := filepath.Join(home, ".zprofile")
	if err := os.WriteFile(zprofile, []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("write .zprofile: %v", err)
	}

	got, err := resolveSetupProfilePath("zsh", home)
	if err != nil {
		t.Fatalf("resolveSetupProfilePath returned error: %v", err)
	}
	if got != zprofile {
		t.Fatalf("expected %q, got %q", zprofile, got)
	}
}

func TestResolveSetupProfilePathBashDefaultsToBashLogin(t *testing.T) {
	home := t.TempDir()
	got, err := resolveSetupProfilePath("bash", home)
	if err != nil {
		t.Fatalf("resolveSetupProfilePath returned error: %v", err)
	}
	want := filepath.Join(home, ".bash_login")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestEnsurePathInShellProfileIdempotent(t *testing.T) {
	profile := filepath.Join(t.TempDir(), ".zshenv")
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir binDir: %v", err)
	}

	changed, already, err := ensurePathInShellProfile(profile, binDir)
	if err != nil {
		t.Fatalf("ensurePathInShellProfile returned error: %v", err)
	}
	if !changed || already {
		t.Fatalf("expected first write to change profile, got changed=%v already=%v", changed, already)
	}

	b, err := os.ReadFile(profile)
	if err != nil {
		t.Fatalf("read profile: %v", err)
	}
	content := string(b)
	if !strings.Contains(content, setupPathBlockStart) || !strings.Contains(content, setupPathBlockEnd) {
		t.Fatalf("expected managed PATH block in profile, got: %q", content)
	}

	changed, already, err = ensurePathInShellProfile(profile, binDir)
	if err != nil {
		t.Fatalf("ensurePathInShellProfile second call returned error: %v", err)
	}
	if changed || !already {
		t.Fatalf("expected idempotent second call, got changed=%v already=%v", changed, already)
	}
}

func TestEnsureSetupShellPathSkipsWhenBinaryPathIsNotStable(t *testing.T) {
	origHomeFn := setupUserHomeDirFn
	origLookPathFn := setupLookPathFn
	origExecFn := setupExecutablePathFn
	origGetenvFn := setupGetenvFn
	defer func() {
		setupUserHomeDirFn = origHomeFn
		setupLookPathFn = origLookPathFn
		setupExecutablePathFn = origExecFn
		setupGetenvFn = origGetenvFn
	}()

	home := t.TempDir()
	setupUserHomeDirFn = func() (string, error) { return home, nil }
	setupGetenvFn = func(key string) string {
		if key == "SHELL" {
			return "/bin/zsh"
		}
		return ""
	}
	setupLookPathFn = func(file string) (string, error) {
		return "", os.ErrNotExist
	}
	setupExecutablePathFn = func() (string, error) {
		return filepath.Join(os.TempDir(), "go-build123", "b001", "exe", "main"), nil
	}

	status, err := ensureSetupShellPath()
	if err != nil {
		t.Fatalf("ensureSetupShellPath returned error: %v", err)
	}
	if strings.TrimSpace(status.SkippedReason) == "" {
		t.Fatalf("expected skipped reason for unstable binary path")
	}
	if status.Changed {
		t.Fatalf("expected no profile changes when skipped")
	}
}
