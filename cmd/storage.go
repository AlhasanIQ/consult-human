package cmd

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/AlhasanIQ/consult-human/config"
)

const storageProviderAll = "all"

type storageClearReport struct {
	Removed []string
	Missing []string
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
	fmt.Fprintln(w, "  consult-human storage clear [--provider all|telegram|whatsapp]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Clears local runtime storage/cache files.")
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
	switch providerName {
	case setupProviderTelegram:
		return telegramStorageTargets()
	case setupProviderWhatsApp:
		return whatsAppStorageTargets(cfg)
	case storageProviderAll:
		tg, err := telegramStorageTargets()
		if err != nil {
			return nil, err
		}
		wa, err := whatsAppStorageTargets(cfg)
		if err != nil {
			return nil, err
		}
		return dedupeNonEmpty(append(tg, wa...)), nil
	default:
		return nil, fmt.Errorf("provider must be all, telegram, or whatsapp")
	}
}

func telegramStorageTargets() ([]string, error) {
	path, err := config.TelegramPendingStorePath()
	if err != nil {
		return nil, err
	}
	return dedupeNonEmpty([]string{
		path,
		path + ".lock",
		path + ".tmp",
	}), nil
}

func whatsAppStorageTargets(cfg config.Config) ([]string, error) {
	storePath := strings.TrimSpace(cfg.WhatsApp.StorePath)
	if storePath == "" {
		defaultPath, err := config.DefaultWhatsAppStorePath()
		if err != nil {
			return nil, err
		}
		storePath = defaultPath
	}
	expanded, err := config.ExpandPath(storePath)
	if err != nil {
		return nil, err
	}
	storePath = strings.TrimSpace(expanded)
	return dedupeNonEmpty([]string{
		storePath,
		storePath + "-wal",
		storePath + "-shm",
		storePath + "-journal",
	}), nil
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
