package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	setupPathBlockStart = "# >>> consult-human PATH >>>"
	setupPathBlockEnd   = "# <<< consult-human PATH <<<"
	setupPathVSCodeNote = "Claude Code VS Code extension may not load ~/.zshrc or ~/.bashrc. Login profiles are used instead: zsh (~/.zshenv, ~/.zprofile, ~/.zlogin), bash (~/.bash_profile, ~/.bash_login)."
)

var (
	setupEnsureShellPathFn = ensureSetupShellPath
	setupUserHomeDirFn     = os.UserHomeDir
	setupExecutablePathFn  = os.Executable
	setupLookPathFn        = exec.LookPath
	setupGetenvFn          = os.Getenv
)

var setupPathBlockPattern = regexp.MustCompile(`(?s)` + regexp.QuoteMeta(setupPathBlockStart) + `\n.*?\n` + regexp.QuoteMeta(setupPathBlockEnd) + `\n?`)

type setupShellPathStatus struct {
	Shell          string
	ProfilePath    string
	BinaryDir      string
	Changed        bool
	AlreadyPresent bool
	SkippedReason  string
}

func runSetupShellPathInteractiveStep(s *sty) {
	if s == nil {
		return
	}
	s.section("Shell PATH")

	status, err := setupEnsureShellPathFn()
	if err != nil {
		s.errMsg("Could not ensure shell PATH automatically: " + err.Error())
		s.info(s.dim(setupPathVSCodeNote))
		return
	}

	renderSetupShellPathStatusStyled(s, status)
	s.info(s.dim(setupPathVSCodeNote))
}

func writeShellPathChecklist(w io.Writer) {
	fmt.Fprintln(w, "Shell PATH:")
	status, err := setupEnsureShellPathFn()
	if err != nil {
		fmt.Fprintf(w, "  Status: could not ensure shell PATH automatically: %v\n", err)
		fmt.Fprintf(w, "  Note: %s\n", setupPathVSCodeNote)
		fmt.Fprintln(w)
		return
	}

	renderSetupShellPathStatusPlain(w, status)
	fmt.Fprintf(w, "  Note: %s\n", setupPathVSCodeNote)
	fmt.Fprintln(w)
}

func renderSetupShellPathStatusStyled(s *sty, status setupShellPathStatus) {
	switch {
	case strings.TrimSpace(status.SkippedReason) != "":
		s.errMsg(status.SkippedReason)
	case status.Changed:
		s.success(fmt.Sprintf("Added %s to PATH via %s", status.BinaryDir, status.ProfilePath))
	case status.AlreadyPresent:
		s.success(fmt.Sprintf("PATH already includes %s via %s", status.BinaryDir, status.ProfilePath))
	default:
		s.errMsg("Shell PATH status is unknown")
	}

	if strings.TrimSpace(status.Shell) != "" {
		s.info(s.dim("Detected shell: " + status.Shell))
	}
	if strings.TrimSpace(status.ProfilePath) != "" {
		s.info(s.dim("Profile file: " + status.ProfilePath))
	}
}

func renderSetupShellPathStatusPlain(w io.Writer, status setupShellPathStatus) {
	switch {
	case strings.TrimSpace(status.SkippedReason) != "":
		fmt.Fprintf(w, "  Status: %s\n", status.SkippedReason)
	case status.Changed:
		fmt.Fprintf(w, "  Status: added %s to PATH via %s\n", status.BinaryDir, status.ProfilePath)
	case status.AlreadyPresent:
		fmt.Fprintf(w, "  Status: PATH already includes %s via %s\n", status.BinaryDir, status.ProfilePath)
	default:
		fmt.Fprintln(w, "  Status: shell PATH status is unknown")
	}

	if strings.TrimSpace(status.Shell) != "" {
		fmt.Fprintf(w, "  Shell: %s\n", status.Shell)
	}
}

func ensureSetupShellPath() (setupShellPathStatus, error) {
	var status setupShellPathStatus

	shell := detectSetupShell()
	if shell == "" {
		status.SkippedReason = "could not detect supported shell from SHELL; supported shells: zsh, bash"
		return status, nil
	}
	status.Shell = shell

	home, err := setupUserHomeDirFn()
	if err != nil {
		return status, err
	}
	profilePath, err := resolveSetupProfilePath(shell, home)
	if err != nil {
		return status, err
	}
	status.ProfilePath = profilePath

	binaryDir, err := resolveSetupBinaryDir()
	if err != nil {
		status.SkippedReason = err.Error()
		return status, nil
	}
	status.BinaryDir = binaryDir

	changed, already, err := ensurePathInShellProfile(profilePath, binaryDir)
	if err != nil {
		return status, err
	}
	status.Changed = changed
	status.AlreadyPresent = already
	return status, nil
}

func detectSetupShell() string {
	raw := strings.TrimSpace(setupGetenvFn("SHELL"))
	if raw == "" {
		return ""
	}
	switch strings.ToLower(filepath.Base(raw)) {
	case "zsh":
		return "zsh"
	case "bash":
		return "bash"
	default:
		return ""
	}
}

func resolveSetupProfilePath(shell, home string) (string, error) {
	if strings.TrimSpace(home) == "" {
		return "", fmt.Errorf("home directory is empty")
	}

	switch strings.TrimSpace(strings.ToLower(shell)) {
	case "zsh":
		for _, name := range []string{".zshenv", ".zprofile", ".zlogin"} {
			p := filepath.Join(home, name)
			exists, err := fileExists(p)
			if err != nil {
				return "", err
			}
			if exists {
				return p, nil
			}
		}
		return filepath.Join(home, ".zshenv"), nil
	case "bash":
		for _, name := range []string{".bash_profile", ".bash_login"} {
			p := filepath.Join(home, name)
			exists, err := fileExists(p)
			if err != nil {
				return "", err
			}
			if exists {
				return p, nil
			}
		}
		return filepath.Join(home, ".bash_login"), nil
	default:
		return "", fmt.Errorf("unsupported shell %q", shell)
	}
}

func resolveSetupBinaryDir() (string, error) {
	type candidate struct {
		path string
	}
	candidates := make([]candidate, 0, 2)
	if p, err := setupLookPathFn("consult-human"); err == nil {
		candidates = append(candidates, candidate{path: p})
	}
	if p, err := setupExecutablePathFn(); err == nil {
		candidates = append(candidates, candidate{path: p})
	}

	seen := map[string]struct{}{}
	for _, c := range candidates {
		normalized, err := normalizeExecutablePath(c.path)
		if err != nil {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}

		if !isStableConsultHumanBinaryPath(normalized) {
			continue
		}
		return filepath.Dir(normalized), nil
	}

	return "", fmt.Errorf("could not determine a stable consult-human binary path (run setup from installed consult-human binary)")
}

func normalizeExecutablePath(raw string) (string, error) {
	p := strings.TrimSpace(raw)
	if p == "" {
		return "", fmt.Errorf("empty executable path")
	}
	if !filepath.IsAbs(p) {
		abs, err := filepath.Abs(p)
		if err != nil {
			return "", err
		}
		p = abs
	}
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		p = resolved
	}
	return filepath.Clean(p), nil
}

func isStableConsultHumanBinaryPath(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	if base != "consult-human" && base != "consult-human.exe" {
		return false
	}

	tmp := filepath.Clean(os.TempDir())
	if tmp != "." && tmp != "" {
		tmpWithSep := tmp + string(filepath.Separator)
		if path == tmp || strings.HasPrefix(path, tmpWithSep) {
			return false
		}
	}

	lowerPath := strings.ToLower(path)
	goBuildFragment := string(filepath.Separator) + "go-build"
	return !strings.Contains(lowerPath, goBuildFragment)
}

func ensurePathInShellProfile(profilePath, binaryDir string) (bool, bool, error) {
	if strings.TrimSpace(profilePath) == "" {
		return false, false, fmt.Errorf("profile path is empty")
	}
	if strings.TrimSpace(binaryDir) == "" {
		return false, false, fmt.Errorf("binary directory is empty")
	}

	content, mode, err := readProfileFile(profilePath)
	if err != nil {
		return false, false, err
	}

	block := buildShellPathBlock(binaryDir)
	updated, found, changed := replaceShellPathManagedBlock(content, block)
	if found {
		if !changed {
			return false, true, nil
		}
		if err := writeFileAtomic(profilePath, updated, mode); err != nil {
			return false, false, err
		}
		return true, false, nil
	}

	if profileAlreadyContainsPath(content, binaryDir) {
		return false, true, nil
	}

	updated = appendShellPathManagedBlock(content, block)
	if err := writeFileAtomic(profilePath, updated, mode); err != nil {
		return false, false, err
	}
	return true, false, nil
}

func buildShellPathBlock(binaryDir string) string {
	quoted := shellSingleQuote(binaryDir)
	return strings.Join([]string{
		setupPathBlockStart,
		fmt.Sprintf("consult_human_bin_dir=%s", quoted),
		"if [ -d \"$consult_human_bin_dir\" ] && [[ \":$PATH:\" != *\":$consult_human_bin_dir:\"* ]]; then",
		"  export PATH=\"$consult_human_bin_dir:$PATH\"",
		"fi",
		"unset consult_human_bin_dir",
		setupPathBlockEnd,
		"",
	}, "\n")
}

func shellSingleQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'\"'\"'`) + "'"
}

func replaceShellPathManagedBlock(content, block string) (string, bool, bool) {
	loc := setupPathBlockPattern.FindStringIndex(content)
	if loc == nil {
		return content, false, false
	}
	current := content[loc[0]:loc[1]]
	if current == block {
		return content, true, false
	}
	updated := setupPathBlockPattern.ReplaceAllString(content, block)
	return updated, true, updated != content
}

func appendShellPathManagedBlock(content, block string) string {
	trimmed := strings.TrimRight(content, "\n")
	if strings.TrimSpace(trimmed) == "" {
		return block
	}
	return trimmed + "\n\n" + block
}

func profileAlreadyContainsPath(content, binaryDir string) bool {
	if strings.TrimSpace(binaryDir) == "" {
		return false
	}
	if !strings.Contains(content, binaryDir) {
		return false
	}
	return strings.Contains(strings.ToLower(content), "path")
}

func readProfileFile(path string) (string, os.FileMode, error) {
	mode := os.FileMode(0o644)
	info, err := os.Stat(path)
	if err == nil {
		mode = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", 0, err
	}

	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", mode, nil
		}
		return "", 0, err
	}
	return string(b), mode, nil
}

func writeFileAtomic(path string, content string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func fileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return !info.IsDir(), nil
}
