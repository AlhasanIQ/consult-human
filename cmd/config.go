package cmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/AlhasanIQ/consult-human/config"
)

func runConfig(args []string, io IO) error {
	if len(args) == 0 {
		printConfigUsage(io.ErrOut)
		return fmt.Errorf("missing config subcommand")
	}

	sub := strings.ToLower(strings.TrimSpace(args[0]))
	subArgs := args[1:]

	switch sub {
	case "path":
		path, err := config.ConfigPath()
		if err != nil {
			return err
		}
		fmt.Fprintln(io.Out, path)
		return nil
	case "show":
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		b, err := config.Marshal(cfg)
		if err != nil {
			return err
		}
		_, err = io.Out.Write(b)
		return err
	case "init":
		path, err := config.ConfigPath()
		if err != nil {
			return err
		}
		_, statErr := os.Stat(path)
		if statErr == nil {
			fmt.Fprintf(io.ErrOut, "Config already exists at %s\n", path)
			return nil
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return statErr
		}
		cfg := config.Default()
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Fprintf(io.ErrOut, "Initialized config at %s\n", path)
		return nil
	case "set":
		if len(subArgs) < 2 {
			printConfigUsage(io.ErrOut)
			return fmt.Errorf("usage: consult-human config set <key> <value>")
		}
		key := subArgs[0]
		value := strings.Join(subArgs[1:], " ")

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if err := config.Set(&cfg, key, value); err != nil {
			return err
		}
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Fprintf(io.ErrOut, "Updated %s\n", key)
		return nil
	case "reset":
		return runConfigReset(subArgs, io)
	case "help", "--help", "-h":
		printConfigUsage(io.Out)
		return nil
	default:
		printConfigUsage(io.ErrOut)
		return fmt.Errorf("unknown config subcommand %q", sub)
	}
}

func printConfigUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  consult-human config path")
	fmt.Fprintln(w, "  consult-human config show")
	fmt.Fprintln(w, "  consult-human config init")
	fmt.Fprintln(w, "  consult-human config set <key> <value>")
	fmt.Fprintln(w, "  consult-human config reset [--provider telegram|whatsapp]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Supported keys:")
	fmt.Fprintln(w, "  default-provider | provider | active_provider")
	fmt.Fprintln(w, "  request_timeout")
	fmt.Fprintln(w, "  telegram.bot_token")
	fmt.Fprintln(w, "  telegram.chat_id")
	fmt.Fprintln(w, "  telegram.poll_interval_seconds")
	fmt.Fprintln(w, "  whatsapp.recipient")
	fmt.Fprintln(w, "  whatsapp.store_path")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Note: whatsapp provider is temporarily disabled.")
}

func runConfigReset(args []string, io IO) error {
	fs := flag.NewFlagSet("config reset", flag.ContinueOnError)
	fs.SetOutput(io.ErrOut)

	var providerName string
	fs.StringVar(&providerName, "provider", "", "Reset only one provider (telegram|whatsapp)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: consult-human config reset [--provider telegram|whatsapp]")
	}

	path, err := config.ConfigPath()
	if err != nil {
		return err
	}

	providerName = strings.ToLower(strings.TrimSpace(providerName))
	if providerName == "" {
		if err := os.Remove(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				fmt.Fprintf(io.ErrOut, "Config not found at %s\n", path)
				return nil
			}
			return err
		}
		fmt.Fprintf(io.ErrOut, "Deleted config at %s\n", path)
		return nil
	}

	if providerName != "telegram" && providerName != "whatsapp" {
		return fmt.Errorf("provider must be telegram or whatsapp")
	}

	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(io.ErrOut, "Config not found at %s\n", path)
			return nil
		}
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	switch providerName {
	case "telegram":
		cfg.Telegram = config.TelegramConfig{}
	case "whatsapp":
		cfg.WhatsApp = config.WhatsAppConfig{}
	}

	telegramConfigured := strings.TrimSpace(cfg.Telegram.BotToken) != ""
	if cfg.ActiveProvider == providerName {
		if providerName == "whatsapp" && telegramConfigured {
			cfg.ActiveProvider = "telegram"
		}
	}
	if cfg.ActiveProvider == "whatsapp" {
		cfg.ActiveProvider = "telegram"
	}

	if err := config.Save(cfg); err != nil {
		return err
	}
	fmt.Fprintf(io.ErrOut, "Reset provider %s\n", providerName)
	return nil
}
