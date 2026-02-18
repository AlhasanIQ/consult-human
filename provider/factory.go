package provider

import (
	"fmt"
	"strings"

	"github.com/AlhasanIQ/consult-human/config"
)

func New(cfg config.Config, override string) (Provider, error) {
	config.ApplyDefaults(&cfg)
	name := strings.ToLower(strings.TrimSpace(override))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(cfg.ActiveProvider))
	}

	switch name {
	case "telegram":
		return NewTelegram(cfg)
	case "whatsapp":
		return nil, fmt.Errorf("whatsapp provider is temporarily disabled")
	default:
		return nil, fmt.Errorf("unknown provider %q", name)
	}
}
