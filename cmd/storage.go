package cmd

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlhasanIQ/consult-human/config"
)

const storageProviderAll = "all"

type storageClearReport struct {
	Removed []string
	Missing []string
}

type telegramStoragePaths struct {
	Pending    string
	Inbox      string
	PollerLock string
}

func runStorage(args []string, io IO) error {
	if len(args) == 0 {
		printStorageUsage(io.ErrOut)
		return fmt.Errorf("missing storage subcommand")
	}

	sub := strings.ToLower(strings.TrimSpace(args[0]))
	subArgs := args[1:]

	switch sub {
	case "clear":
		return runStorageClear(subArgs, io)
	case "path":
		return runStoragePath(subArgs, io)
	case "help", "--help", "-h":
		printStorageUsage(io.Out)
		return nil
	default:
		printStorageUsage(io.ErrOut)
		return fmt.Errorf("unknown storage subcommand %q", sub)
	}
}

func printStorageUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  consult-human storage path [--provider all|telegram|whatsapp]")
	fmt.Fprintln(w, "  consult-human storage clear [--provider all|telegram|whatsapp]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Shows or clears local runtime storage/cache files.")
}

func runStorageClear(args []string, io IO) error {
	fs := flag.NewFlagSet("storage clear", flag.ContinueOnError)
	fs.SetOutput(io.ErrOut)

	var providerName string
	fs.StringVar(&providerName, "provider", storageProviderAll, "Provider scope to clear (all|telegram|whatsapp)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: consult-human storage clear [--provider all|telegram|whatsapp]")
	}

	providerName, err := normalizeStorageProvider(providerName)
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	report, err := clearStorageWithConfig(cfg, providerName)
	if err != nil {
		return err
	}
	printStorageClearReport(io.ErrOut, providerName, report)
	return nil
}

func runStoragePath(args []string, io IO) error {
	fs := flag.NewFlagSet("storage path", flag.ContinueOnError)
	fs.SetOutput(io.ErrOut)

	var providerName string
	fs.StringVar(&providerName, "provider", storageProviderAll, "Provider scope (all|telegram|whatsapp)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: consult-human storage path [--provider all|telegram|whatsapp]")
	}

	providerName, err := normalizeStorageProvider(providerName)
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	tgPaths, err := effectiveTelegramStoragePaths(cfg)
	if err != nil {
		return err
	}
	waPath, err := effectiveWhatsAppStorePath(cfg)
	if err != nil {
		return err
	}
	if providerName == setupProviderTelegram || providerName == setupProviderWhatsApp {
		if providerName == setupProviderTelegram {
			fmt.Fprintf(io.Out, "pending: %s\n", tgPaths.Pending)
			fmt.Fprintf(io.Out, "inbox: %s\n", tgPaths.Inbox)
		} else {
			fmt.Fprintln(io.Out, waPath)
		}
		return nil
	}

	fmt.Fprintf(io.Out, "telegram.pending: %s\n", tgPaths.Pending)
	fmt.Fprintf(io.Out, "telegram.inbox: %s\n", tgPaths.Inbox)
	fmt.Fprintf(io.Out, "whatsapp: %s\n", waPath)
	return nil
}

func normalizeStorageProvider(raw string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(raw))
	if name == "" {
		name = storageProviderAll
	}
	switch name {
	case storageProviderAll, setupProviderTelegram, setupProviderWhatsApp:
		return name, nil
	default:
		return "", fmt.Errorf("provider must be all, telegram, or whatsapp")
	}
}

func clearStorageWithConfig(cfg config.Config, providerName string) (storageClearReport, error) {
	targets, err := storageTargetsForProvider(cfg, providerName)
	if err != nil {
		return storageClearReport{}, err
	}

	report := storageClearReport{
		Removed: make([]string, 0, len(targets)),
		Missing: make([]string, 0, len(targets)),
	}
	for _, path := range targets {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				report.Missing = append(report.Missing, path)
				continue
			}
			return report, fmt.Errorf("delete %s: %w", path, err)
		}
		report.Removed = append(report.Removed, path)
	}
	return report, nil
}

func storageTargetsForProvider(cfg config.Config, providerName string) ([]string, error) {
	tgPaths, err := effectiveTelegramStoragePaths(cfg)
	if err != nil {
		return nil, err
	}
	waPath, err := effectiveWhatsAppStorePath(cfg)
	if err != nil {
		return nil, err
	}

	switch providerName {
	case setupProviderTelegram:
		return telegramStorageTargets(tgPaths), nil
	case setupProviderWhatsApp:
		return whatsAppStorageTargets(waPath), nil
	case storageProviderAll:
		tg := telegramStorageTargets(tgPaths)
		wa := whatsAppStorageTargets(waPath)
		return dedupeNonEmpty(append(tg, wa...)), nil
	default:
		return nil, fmt.Errorf("provider must be all, telegram, or whatsapp")
	}
}

func effectiveTelegramStoragePaths(cfg config.Config) (telegramStoragePaths, error) {
	pendingPath, err := config.EffectiveTelegramPendingStorePath(cfg)
	if err != nil {
		return telegramStoragePaths{}, err
	}
	inboxPath, err := config.EffectiveTelegramInboxStorePath(cfg)
	if err != nil {
		return telegramStoragePaths{}, err
	}
	return telegramStoragePaths{
		Pending:    pendingPath,
		Inbox:      inboxPath,
		PollerLock: filepath.Join(filepath.Dir(inboxPath), "telegram-poller.lock"),
	}, nil
}

func telegramStorageTargets(paths telegramStoragePaths) []string {
	return dedupeNonEmpty([]string{
		paths.Pending,
		paths.Pending + ".lock",
		paths.Pending + ".tmp",
		paths.Inbox,
		paths.Inbox + ".lock",
		paths.Inbox + ".tmp",
		paths.PollerLock,
	})
}

func effectiveWhatsAppStorePath(cfg config.Config) (string, error) {
	storePath := strings.TrimSpace(cfg.WhatsApp.StorePath)
	if storePath == "" {
		defaultPath, err := config.DefaultWhatsAppStorePath()
		if err != nil {
			return "", err
		}
		storePath = defaultPath
	}
	expanded, err := config.ExpandPath(storePath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(expanded), nil
}

func whatsAppStorageTargets(storePath string) []string {
	return dedupeNonEmpty([]string{
		storePath,
		storePath + "-wal",
		storePath + "-shm",
		storePath + "-journal",
	})
}

func dedupeNonEmpty(paths []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func printStorageClearReport(w io.Writer, providerName string, report storageClearReport) {
	for _, p := range report.Removed {
		fmt.Fprintf(w, "Deleted %s\n", p)
	}
	if len(report.Removed) == 0 {
		fmt.Fprintf(w, "No storage files found for %s\n", providerName)
		return
	}
	fmt.Fprintf(w, "Cleared storage for %s\n", providerName)
}
