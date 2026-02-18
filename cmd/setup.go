package cmd

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/AlhasanIQ/consult-human/config"
)

const (
	setupProviderTelegram = "telegram"
	setupProviderWhatsApp = "whatsapp"
)

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
	if selectedExplicit {
		if err := validateSetupSelection(cfg, selected); err != nil {
			return err
		}
	}

	path, err := config.ConfigPath()
	if err != nil {
		return err
	}

	fmt.Fprintln(w, "Setup checklist (non-interactive)")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Config path: %s\n", path)
	fmt.Fprintln(w)

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
	return nil
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
