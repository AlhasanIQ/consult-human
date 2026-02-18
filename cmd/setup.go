package cmd

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AlhasanIQ/consult-human/config"
)

const (
	setupProviderTelegram = "telegram"
	setupProviderWhatsApp = "whatsapp"
)

var setupSkillInstallFn = runSkillInstall
var setupCurrentDirFn = os.Getwd

func runSetup(args []string, io IO) error {
	if isSetupHelpRequest(args) {
		printSetupUsage(io.Out)
		return nil
	}

	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(io.ErrOut)
	fs.Usage = func() { printSetupUsage(io.ErrOut) }

	var nonInteractive bool
	var providersRaw stringSliceFlag
	fs.BoolVar(&nonInteractive, "non-interactive", false, "Print setup checklist instead of prompting")
	fs.Var(&providersRaw, "provider", "Provider to include (telegram). Repeatable.")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		printSetupUsage(io.ErrOut)
		return fmt.Errorf("setup does not take positional arguments")
	}

	selected, err := parseSetupProviderFlags([]string(providersRaw))
	if err != nil {
		return err
	}
	selectedExplicit := len(selected) > 0
	if !selectedExplicit {
		selected = []string{setupProviderTelegram}
	}
	if err := validateSetupProvidersEnabled(selected); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if nonInteractive {
		return runSetupNonInteractive(io.Out, cfg, selected, selectedExplicit)
	}
	return runSetupInteractive(io, cfg, selected)
}

func isSetupHelpRequest(args []string) bool {
	if len(args) > 0 && strings.EqualFold(strings.TrimSpace(args[0]), "help") {
		return true
	}
	for _, arg := range args {
		switch strings.TrimSpace(strings.ToLower(arg)) {
		case "--help", "-h":
			return true
		}
	}
	return false
}

func runSetupInteractive(io IO, cfg config.Config, selected []string) error {
	s := newSty(io.ErrOut)
	s.header("consult-human · interactive setup")
	s.info(s.dim("WhatsApp is temporarily disabled. Configuring Telegram only."))
	runSetupShellPathInteractiveStep(s)

	if err := validateSetupSelection(cfg, selected); err != nil {
		return err
	}

	reader := bufio.NewReader(io.In)
	for _, providerName := range selected {
		switch providerName {
		case setupProviderTelegram:
			if err := runTelegramSetup(reader, s, &cfg); err != nil {
				return err
			}
		}
	}

	cfg.ActiveProvider = setupProviderTelegram
	if err := config.Save(cfg); err != nil {
		return err
	}

	if err := runSetupSkillInstallInteractive(reader, s, io); err != nil {
		return err
	}

	path, err := config.ConfigPath()
	if err != nil {
		return err
	}

	fmt.Fprintln(s.w)
	s.success("Setup complete")
	s.info(s.dim(fmt.Sprintf("Config saved to %s", path)))
	return nil
}

func runSetupNonInteractive(w io.Writer, cfg config.Config, selected []string, selectedExplicit bool) error {
	path, err := config.ConfigPath()
	if err != nil {
		return err
	}

	fmt.Fprintln(w, "Setup checklist (non-interactive)")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Config path: %s\n", path)
	fmt.Fprintln(w)
	writeShellPathChecklist(w)

	if selectedExplicit {
		if err := validateSetupSelection(cfg, selected); err != nil {
			return err
		}
	}

	for _, providerName := range selected {
		switch providerName {
		case setupProviderTelegram:
			writeTelegramChecklist(w, isProviderSetupComplete(cfg, setupProviderTelegram))
		}
	}

	if !selectedExplicit {
		writeWhatsAppDeferredNotice(w, cfg)
	}

	fmt.Fprintln(w, "Next: set active provider with `consult-human config set default-provider telegram`.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Verification: run `consult-human config show`.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Final step: install skill files for agent runtimes (required):")
	fmt.Fprintln(w, "  consult-human skill install --target claude")
	fmt.Fprintln(w, "  consult-human skill install --target codex")
	fmt.Fprintln(w, "  consult-human skill install --target both")
	fmt.Fprintln(w, "  consult-human skill install --target claude --repo /path/to/repo")
	return nil
}

func runSetupSkillInstallInteractive(reader *bufio.Reader, s *sty, runtimeIO IO) error {
	s.section("Skill Installation")
	s.info("Choose where to install the consult-human skill.")
	s.info("You can choose one or multiple targets (comma-separated).")
	fmt.Fprintln(s.w)
	s.choice(1, "claude", "(Cloud/Claude Code)")
	s.choice(2, "codex", "(Codex CLI)")
	s.choice(3, "both", "(install to both targets)")
	fmt.Fprintln(s.w)

	for {
		selectionRaw, err := promptRequiredLine(reader, s, s.promptLabel("Skill targets (e.g. 1 or 1,2): "))
		if err != nil {
			return err
		}
		target, err := parseSetupSkillTargetSelection(selectionRaw)
		if err != nil {
			s.errMsg(err.Error())
			continue
		}

		installArgs, err := promptSetupSkillInstallScope(reader, s, target)
		if err != nil {
			return err
		}

		args := append([]string{"--target", target}, installArgs...)
		if err := setupSkillInstallFn(args, runtimeIO); err != nil {
			return fmt.Errorf("skill install failed: %w", err)
		}
		fmt.Fprintln(s.w)
		s.success(fmt.Sprintf("Skill installed for %s", target))
		return nil
	}
}

type setupSkillScopeOption struct {
	Token        string
	Label        string
	Description  string
	RepoRoot     string
	Destinations []string
	Notes        []string
}

func promptSetupSkillInstallScope(reader *bufio.Reader, s *sty, target string) ([]string, error) {
	options, defaultToken, err := buildSetupSkillScopeOptions(target)
	if err != nil {
		return nil, err
	}
	if len(options) == 0 {
		return nil, fmt.Errorf("no install scope options available")
	}

	fmt.Fprintln(s.w)
	s.info("Choose install scope:")
	for i, opt := range options {
		note := opt.Description
		if opt.Token == defaultToken {
			note = note + " (default)"
		}
		s.choice(i+1, opt.Label, note)
		for _, dst := range opt.Destinations {
			s.info("      path: " + dst)
		}
		for _, n := range opt.Notes {
			s.info("      note: " + n)
		}
	}
	fmt.Fprintln(s.w)

	prompt := s.promptLabel(fmt.Sprintf("Install scope [%s]: ", defaultToken))
	selectionRaw, err := promptLine(reader, s.w, prompt)
	if err != nil {
		return nil, err
	}
	selection := strings.ToLower(strings.TrimSpace(selectionRaw))
	if selection == "" {
		selection = defaultToken
	}

	opt, err := selectSetupSkillScopeOption(options, selection)
	if err != nil {
		return nil, err
	}
	if opt.RepoRoot == "__custom__" {
		customPathRaw, err := promptRequiredLine(reader, s, s.promptLabel("Custom repo path: "))
		if err != nil {
			return nil, err
		}
		customPath, err := resolveRepoRoot(customPathRaw)
		if err != nil {
			return nil, err
		}
		dests, notes, err := resolveSkillDestinations(target, customPath)
		if err != nil {
			return nil, err
		}
		fmt.Fprintln(s.w)
		s.info("Installing into custom repo path:")
		for _, dst := range dests {
			s.info("      path: " + dst)
		}
		for _, n := range notes {
			s.info("      note: " + n)
		}
		return []string{"--repo", customPath}, nil
	}
	if strings.TrimSpace(opt.RepoRoot) != "" {
		return []string{"--repo", opt.RepoRoot}, nil
	}
	return nil, nil
}

func selectSetupSkillScopeOption(options []setupSkillScopeOption, selection string) (setupSkillScopeOption, error) {
	for i, opt := range options {
		numberToken := strconv.Itoa(i + 1)
		if selection == numberToken || selection == opt.Token {
			return opt, nil
		}
	}
	return setupSkillScopeOption{}, fmt.Errorf("invalid install scope %q", selection)
}

func buildSetupSkillScopeOptions(target string) ([]setupSkillScopeOption, string, error) {
	globalDests, globalNotes, err := resolveSkillDestinations(target, "")
	if err != nil {
		return nil, "", err
	}

	options := []setupSkillScopeOption{
		{
			Token:        "global",
			Label:        "Global (user account)",
			Description:  "Install for all Cloud/Claude Code and Codex sessions in this user account on this machine.",
			RepoRoot:     "",
			Destinations: globalDests,
			Notes:        globalNotes,
		},
	}
	defaultToken := "global"

	repoRoot, repoDetected, repoHasAgentDirs, err := detectCurrentRepoForSkillSetup()
	if err != nil {
		return nil, "", err
	}
	if repoDetected {
		repoDests, repoNotes, err := resolveSkillDestinations(target, repoRoot)
		if err != nil {
			return nil, "", err
		}
		options = append(options, setupSkillScopeOption{
			Token:        "repo",
			Label:        "Current repo",
			Description:  fmt.Sprintf("Install for this repository only (%s).", repoRoot),
			RepoRoot:     repoRoot,
			Destinations: repoDests,
			Notes:        repoNotes,
		})
		if repoHasAgentDirs {
			defaultToken = "repo"
		}
	}

	options = append(options, setupSkillScopeOption{
		Token:        "custom",
		Label:        "Custom repo path",
		Description:  "Specify a repository path manually.",
		RepoRoot:     "__custom__",
		Destinations: skillDestinationTemplates(target),
	})

	return options, defaultToken, nil
}

func skillDestinationTemplates(target string) []string {
	switch target {
	case skillTargetClaude:
		return []string{
			"<custom-repo>/.claude/skills/consult-human/SKILL.md",
		}
	case skillTargetCodex:
		return []string{
			"<custom-repo>/.codex/skills/consult-human/SKILL.md",
		}
	default:
		return []string{
			"<custom-repo>/.claude/skills/consult-human/SKILL.md",
			"<custom-repo>/.codex/skills/consult-human/SKILL.md",
		}
	}
}

func detectCurrentRepoForSkillSetup() (string, bool, bool, error) {
	cwd, err := setupCurrentDirFn()
	if err != nil {
		return "", false, false, err
	}
	repoRoot, found, err := findGitRepoRoot(cwd)
	if err != nil {
		return "", false, false, err
	}
	if !found {
		return "", false, false, nil
	}

	hasClaudeDir, err := dirExists(filepath.Join(repoRoot, ".claude"))
	if err != nil {
		return "", false, false, err
	}
	hasAgentsDir, err := dirExists(filepath.Join(repoRoot, ".agents"))
	if err != nil {
		return "", false, false, err
	}
	return repoRoot, true, hasClaudeDir || hasAgentsDir, nil
}

func findGitRepoRoot(startPath string) (string, bool, error) {
	if strings.TrimSpace(startPath) == "" {
		return "", false, fmt.Errorf("empty start path")
	}
	current := filepath.Clean(startPath)
	for {
		gitPath := filepath.Join(current, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return current, true, nil
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", false, err
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", false, nil
		}
		current = parent
	}
}

func dirExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return info.IsDir(), nil
}

func writeTelegramChecklist(w io.Writer, alreadySetup bool) {
	step := 1
	if alreadySetup {
		fmt.Fprintln(w, "Telegram (already set up):")
		fmt.Fprintln(w, "  Status: already configured.")
		fmt.Fprintln(w, "  Reconfigure first: `consult-human config reset --provider telegram`.")
		fmt.Fprintln(w, "  Fresh setup steps (if reconfiguring):")
	} else {
		fmt.Fprintln(w, "Telegram:")
	}
	fmt.Fprintf(w, "  Step %d: In Telegram, open @BotFather, run /newbot, and copy BOT_TOKEN.\n", step)
	step++
	fmt.Fprintf(w, "  Step %d: Run `consult-human config set telegram.bot_token \"<BOT_TOKEN>\"`.\n", step)
	step++
	fmt.Fprintf(w, "  Step %d: Run `consult-human setup --provider telegram` to link chat via /start.\n", step)
	fmt.Fprintln(w)
}

func writeWhatsAppDeferredNotice(w io.Writer, cfg config.Config) {
	alreadySetup := isProviderSetupComplete(cfg, setupProviderWhatsApp)
	if alreadySetup {
		fmt.Fprintln(w, "WhatsApp (temporarily disabled):")
		fmt.Fprintln(w, "  Status: already configured locally, but disabled for now.")
	} else {
		fmt.Fprintln(w, "WhatsApp (temporarily disabled):")
		fmt.Fprintln(w, "  Status: deferred until later roadmap phase.")
	}
	fmt.Fprintln(w, "  Reason: provider disabled due stability issues; will be re-enabled later.")
	fmt.Fprintln(w)
}

func printSetupUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  consult-human setup [--provider telegram]")
	fmt.Fprintln(w, "  consult-human setup --non-interactive [--provider telegram]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Interactive first-time setup, or checklist-only mode.")
	fmt.Fprintln(w, "Both setup modes ensure consult-human binary PATH in your shell login profile.")
	fmt.Fprintln(w, "WhatsApp is temporarily disabled.")
}

func parseSetupProviderFlags(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	selected := make([]string, 0, len(raw))
	seen := map[string]struct{}{}

	for _, item := range raw {
		for _, part := range strings.Split(item, ",") {
			token := strings.TrimSpace(strings.ToLower(part))
			if token == "" {
				continue
			}
			providerName, err := parseSetupProviderToken(token)
			if err != nil {
				return nil, err
			}
			if _, ok := seen[providerName]; ok {
				continue
			}
			seen[providerName] = struct{}{}
			selected = append(selected, providerName)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("selection is required")
	}
	return selected, nil
}

func parseSetupProviderToken(token string) (string, error) {
	switch token {
	case "1", setupProviderTelegram:
		return setupProviderTelegram, nil
	case "2", setupProviderWhatsApp:
		return "", fmt.Errorf("whatsapp is temporarily disabled")
	default:
		if _, err := strconv.Atoi(token); err == nil {
			return "", fmt.Errorf("unsupported option %q", token)
		}
		return "", fmt.Errorf("unsupported provider %q", token)
	}
}

func parseSetupSkillTargetSelection(raw string) (string, error) {
	claudeSelected := false
	codexSelected := false

	for _, part := range strings.Split(raw, ",") {
		token := strings.TrimSpace(strings.ToLower(part))
		if token == "" {
			continue
		}
		target, err := parseSetupSkillTargetToken(token)
		if err != nil {
			return "", err
		}
		switch target {
		case skillTargetBoth:
			claudeSelected = true
			codexSelected = true
		case skillTargetClaude:
			claudeSelected = true
		case skillTargetCodex:
			codexSelected = true
		}
	}

	if !claudeSelected && !codexSelected {
		return "", fmt.Errorf("select at least one target: claude, codex, or both")
	}
	if claudeSelected && codexSelected {
		return skillTargetBoth, nil
	}
	if claudeSelected {
		return skillTargetClaude, nil
	}
	return skillTargetCodex, nil
}

func parseSetupSkillTargetToken(token string) (string, error) {
	switch token {
	case "1", "claude", "cloud", "cloud-code", "claude-code":
		return skillTargetClaude, nil
	case "2", "codex":
		return skillTargetCodex, nil
	case "3", "both", "all":
		return skillTargetBoth, nil
	default:
		return "", fmt.Errorf("invalid skill target %q", token)
	}
}

func validateSetupSelection(cfg config.Config, selected []string) error {
	for _, providerName := range selected {
		if !isProviderSetupComplete(cfg, providerName) {
			continue
		}
		return fmt.Errorf("%s is already set up. Run `consult-human config reset --provider %s` first", providerName, providerName)
	}
	return nil
}

func isProviderSetupComplete(cfg config.Config, providerName string) bool {
	switch strings.ToLower(strings.TrimSpace(providerName)) {
	case setupProviderTelegram:
		return strings.TrimSpace(cfg.Telegram.BotToken) != "" && cfg.Telegram.ChatID != 0
	case setupProviderWhatsApp:
		return strings.TrimSpace(cfg.WhatsApp.Recipient) != ""
	default:
		return false
	}
}

func validateSetupProvidersEnabled(selected []string) error {
	for _, providerName := range selected {
		if isSetupProviderEnabled(providerName) {
			continue
		}
		return fmt.Errorf("%s is temporarily disabled", providerName)
	}
	return nil
}

func isSetupProviderEnabled(providerName string) bool {
	switch strings.ToLower(strings.TrimSpace(providerName)) {
	case setupProviderTelegram:
		return true
	case setupProviderWhatsApp:
		return false
	default:
		return false
	}
}

func promptRequiredLine(reader *bufio.Reader, s *sty, prompt string) (string, error) {
	for {
		line, err := promptLine(reader, s.w, prompt)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(line) == "" {
			s.errMsg("This value is required.")
			continue
		}
		return strings.TrimSpace(line), nil
	}
}

func promptLine(reader *bufio.Reader, w io.Writer, prompt string) (string, error) {
	fmt.Fprint(w, prompt)
	line, err := reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			return strings.TrimSpace(line), nil
		}
		return "", err
	}
	return strings.TrimSpace(line), nil
}
