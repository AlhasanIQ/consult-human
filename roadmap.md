# Roadmap

## Phase 1: MVP — Blocking CLI with Telegram & WhatsApp

- [ ] Initialize Go module with cobra CLI framework
- [ ] Config package — YAML-based config at `~/.config/consult-human/config.yaml`
- [ ] Provider interface — `Send` and `Receive` in `provider/provider.go`
- [ ] Telegram provider — Bot API integration (sendMessage + getUpdates long polling)
- [ ] WhatsApp provider — Twilio API integration (or whatsmeow, TBD)
- [ ] `ask` command — send question, block until reply, print reply to stdout
- [ ] `config` command — set provider, credentials, chat IDs
- [ ] Skill file for agent instructions
- [ ] README with setup guide

## Phase 2: Async & Agent Ergonomics

- [ ] Research how Claude Code and Codex handle blocking vs non-blocking CLI calls
- [ ] Evaluate token efficiency of blocking vs async patterns
- [ ] Implement async mode if beneficial: `send` / `poll` / `wait` subcommands
- [ ] Local state store for pending requests (file-based or SQLite)
- [ ] Timeout support with configurable defaults

## Phase 3: More Providers & Polish

- [ ] Slack provider
- [ ] Discord provider
- [ ] Email provider (for non-urgent consultations)
- [ ] Provider auto-detection / fallback chain
- [ ] Message formatting (context, urgency levels)
- [ ] Reply confirmation / acknowledgment

## Phase 4: Distribution & Ecosystem

- [ ] Homebrew formula
- [ ] Go install support (`go install github.com/AlhasanIQ/consult-human@latest`)
- [ ] Pre-built binaries (goreleaser)
- [ ] MCP server mode (native Claude Code tool integration)
- [ ] Published skill file for Claude Code marketplace (if applicable)
