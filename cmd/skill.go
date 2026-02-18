package cmd

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlhasanIQ/consult-human/config"
)

const (
	skillSubcommandInstall = "install"

	skillTargetClaude = "claude"
	skillTargetCodex  = "codex"
	skillTargetBoth   = "both"

	skillFileName = "SKILL.md"

	claudeInstructionsFileName = "CLAUDE.md"
	agentsInstructionsFileName = "AGENTS.md"

	consultHumanReminderStart = "<!-- consult-human reminder:start -->"
	consultHumanReminderEnd   = "<!-- consult-human reminder:end -->"
	consultHumanReminderBody  = "## IMPORTANT: Never Forget Human Consultation\nNever forget: before any high-leverage decision, risky or destructive opearation, side-effecting operation, tech-debr inducing design/desision or whenever you are unsure, consult the human with `/consult-human` skill before proceeding. Always check the `/consult-human` skill before consulting.\nWhenever you want to ask/discuss/talk to the human (user), use this skill. Whenever you are unsure about next steps, use this skill."
)

func runSkill(args []string, io IO) error {
	if len(args) == 0 {
		printSkillUsage(io.ErrOut)
		return fmt.Errorf("missing skill subcommand")
	}

	sub := strings.ToLower(strings.TrimSpace(args[0]))
	subArgs := args[1:]
	switch sub {
	case skillSubcommandInstall:
		return runSkillInstall(subArgs, io)
	case "help", "--help", "-h":
		printSkillUsage(io.Out)
		return nil
	default:
		printSkillUsage(io.ErrOut)
		return fmt.Errorf("unknown skill subcommand %q", sub)
	}
}

func printSkillUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  consult-human skill install [--target claude|codex|both] [--repo <path>] [--copy] [--source <SKILL.md path>]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Install consult-human SKILL.md into local agent skill directories.")
}

func runSkillInstall(args []string, io IO) error {
	fs := flag.NewFlagSet("skill install", flag.ContinueOnError)
	fs.SetOutput(io.ErrOut)

	var targetRaw string
	var sourceRaw string
	var repoRaw string
	var linkMode bool
	var copyMode bool

	fs.StringVar(&targetRaw, "target", "", "Install target (claude|codex|both). Defaults to both.")
	fs.StringVar(&sourceRaw, "source", "", "Local SKILL.md source path (optional)")
	fs.StringVar(&repoRaw, "repo", "", "Install inside this repo path instead of user-global directories")
	fs.BoolVar(&linkMode, "link", true, "Symlink SKILL.md instead of copying file contents (default true)")
	fs.BoolVar(&copyMode, "copy", false, "Copy SKILL.md instead of symlinking")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: consult-human skill install [--target claude|codex|both] [--repo <path>] [--copy] [--source <SKILL.md path>]")
	}
	if copyMode {
		linkMode = false
	}

	if strings.TrimSpace(targetRaw) == "" {
		targetRaw = skillTargetBoth
	}
	target, err := normalizeSkillTarget(targetRaw)
	if err != nil {
		return err
	}

	sourceBytes, sourcePath, sourceLabel, err := loadSkillSource(strings.TrimSpace(sourceRaw))
	if err != nil {
		return err
	}
	repoRoot, err := resolveRepoRoot(repoRaw)
	if err != nil {
		return err
	}

	destinations, notes, err := resolveSkillDestinations(target, repoRoot)
	if err != nil {
		return err
	}
	if len(destinations) == 0 {
		return fmt.Errorf("no install destinations resolved")
	}

	mode := "copy"
	if linkMode {
		mode = "symlink"
	}
	scope := "global"
	if repoRoot != "" {
		scope = "repo: " + repoRoot
	}
	fmt.Fprintf(io.ErrOut, "Installing skill (%s mode, %s) from %s\n", mode, scope, sourceLabel)
	for _, dst := range destinations {
		targetFile, err := installSkillFile(dst, sourceBytes, sourcePath, linkMode)
		if err != nil {
			return fmt.Errorf("install to %s: %w", dst, err)
		}
		fmt.Fprintf(io.ErrOut, "Installed %s\n", targetFile)
	}
	for _, note := range notes {
		fmt.Fprintf(io.ErrOut, "Note: %s\n", note)
	}

	reminderTargets, err := resolveInstructionReminderTargets(target, repoRoot)
	if err != nil {
		return err
	}
	for _, reminderFile := range reminderTargets {
		changed, err := ensureConsultHumanReminder(reminderFile)
		if err != nil {
			return fmt.Errorf("update reminder in %s: %w", reminderFile, err)
		}
		if changed {
			fmt.Fprintf(io.ErrOut, "Updated agent reminder in %s\n", reminderFile)
		} else {
			fmt.Fprintf(io.ErrOut, "Agent reminder already present in %s\n", reminderFile)
		}
	}
	return nil
}

func normalizeSkillTarget(raw string) (string, error) {
	target := strings.ToLower(strings.TrimSpace(raw))
	switch target {
	case "cloud", "cloud-code", "claude-code":
		return skillTargetClaude, nil
	case skillTargetClaude, skillTargetCodex, skillTargetBoth, "all":
		if target == "all" {
			return skillTargetBoth, nil
		}
		return target, nil
	default:
		return "", fmt.Errorf("target must be claude, codex, or both")
	}
}

func resolveRepoRoot(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	root, err := config.ExpandPath(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("--repo must be a directory")
	}
	return root, nil
}

func resolveSkillDestinations(target string, repoRoot string) ([]string, []string, error) {
	baseRoot := repoRoot
	if baseRoot == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, nil, err
		}
		baseRoot = home
	}

	added := map[string]struct{}{}
	destinations := make([]string, 0, 3)
	add := func(path string) {
		if path == "" {
			return
		}
		if _, ok := added[path]; ok {
			return
		}
		added[path] = struct{}{}
		destinations = append(destinations, path)
	}

	var notes []string
	if target == skillTargetClaude || target == skillTargetBoth {
		add(filepath.Join(baseRoot, ".claude", "skills", "consult-human"))

		agentsRoot := filepath.Join(baseRoot, ".agents")
		if info, statErr := os.Stat(agentsRoot); statErr == nil && info.IsDir() {
			add(filepath.Join(agentsRoot, "skills", "consult-human"))
		}
	}

	if target == skillTargetCodex || target == skillTargetBoth {
		add(filepath.Join(baseRoot, ".codex", "skills", "consult-human"))
	}

	return destinations, notes, nil
}

func resolveInstructionReminderTargets(target string, repoRoot string) ([]string, error) {
	baseRoot := repoRoot
	if baseRoot == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		baseRoot = home
	}

	added := map[string]struct{}{}
	targets := make([]string, 0, 3)
	add := func(path string) {
		if path == "" {
			return
		}
		if _, ok := added[path]; ok {
			return
		}
		added[path] = struct{}{}
		targets = append(targets, path)
	}

	if target == skillTargetClaude || target == skillTargetBoth {
		add(filepath.Join(baseRoot, ".claude", claudeInstructionsFileName))

		agentsRoot := filepath.Join(baseRoot, ".agents")
		if info, statErr := os.Stat(agentsRoot); statErr == nil && info.IsDir() {
			add(filepath.Join(agentsRoot, agentsInstructionsFileName))
		}
	}
	if target == skillTargetCodex || target == skillTargetBoth {
		add(filepath.Join(baseRoot, ".codex", agentsInstructionsFileName))
	}

	return targets, nil
}

func ensureConsultHumanReminder(path string) (bool, error) {
	if strings.TrimSpace(path) == "" {
		return false, fmt.Errorf("empty reminder path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}

	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return false, statErr
	}

	currentBytes, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	current := string(currentBytes)
	desiredBlock := strings.Join([]string{
		consultHumanReminderStart,
		consultHumanReminderBody,
		consultHumanReminderEnd,
	}, "\n")

	updated, changed := upsertConsultHumanReminderBlock(current, desiredBlock)
	if !changed {
		return false, nil
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(updated), mode); err != nil {
		return false, err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return false, err
	}
	return true, nil
}

func upsertConsultHumanReminderBlock(content string, desiredBlock string) (string, bool) {
	start := strings.Index(content, consultHumanReminderStart)
	end := strings.Index(content, consultHumanReminderEnd)
	if start >= 0 && end >= start {
		end += len(consultHumanReminderEnd)
		existing := content[start:end]
		if existing == desiredBlock {
			return content, false
		}
		updated := content[:start] + desiredBlock + content[end:]
		return updated, true
	}

	trimmed := strings.TrimRight(content, "\n")
	if strings.TrimSpace(trimmed) == "" {
		return desiredBlock + "\n", true
	}
	return trimmed + "\n\n" + desiredBlock + "\n", true
}

func installSkillFile(destinationDir string, sourceBytes []byte, sourcePath string, linkMode bool) (string, error) {
	if err := os.MkdirAll(destinationDir, 0o755); err != nil {
		return "", err
	}

	targetFile := filepath.Join(destinationDir, skillFileName)
	if linkMode {
		absSourcePath, err := filepath.Abs(sourcePath)
		if err != nil {
			return "", err
		}
		if err := os.Remove(targetFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if err := os.Symlink(absSourcePath, targetFile); err != nil {
			return "", err
		}
		return targetFile, nil
	}

	tmpFile := targetFile + ".tmp"
	if err := os.WriteFile(tmpFile, sourceBytes, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmpFile, targetFile); err != nil {
		_ = os.Remove(tmpFile)
		return "", err
	}
	return targetFile, nil
}

func loadSkillSource(sourcePathRaw string) ([]byte, string, string, error) {
	if strings.TrimSpace(sourcePathRaw) != "" {
		path, err := config.ExpandPath(sourcePathRaw)
		if err != nil {
			return nil, "", "", err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, "", "", err
		}
		if strings.TrimSpace(string(b)) == "" {
			return nil, "", "", fmt.Errorf("empty skill source: %s", path)
		}
		return b, path, path, nil
	}

	managedPath, err := defaultManagedSkillSourcePath()
	if err != nil {
		return nil, "", "", err
	}
	if err := os.MkdirAll(filepath.Dir(managedPath), 0o755); err != nil {
		return nil, "", "", err
	}

	embedded := bytes.TrimSpace(skillTemplateEmbedded)
	if len(embedded) == 0 {
		return nil, "", "", fmt.Errorf("embedded skill template is empty")
	}

	existing, readErr := os.ReadFile(managedPath)
	if readErr == nil {
		trimmedExisting := bytes.TrimSpace(existing)
		if bytes.Equal(trimmedExisting, embedded) {
			return existing, managedPath, managedPath, nil
		}
	}
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return nil, "", "", readErr
	}

	status := "seeded from embedded template"
	if readErr == nil {
		status = "updated from embedded template"
	}
	if err := os.WriteFile(managedPath, skillTemplateEmbedded, 0o644); err != nil {
		return nil, "", "", err
	}
	return skillTemplateEmbedded, managedPath, fmt.Sprintf("%s (%s)", managedPath, status), nil
}

func defaultManagedSkillSourcePath() (string, error) {
	cfgPath, err := config.ConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(cfgPath), skillFileName), nil
}
