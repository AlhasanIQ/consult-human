# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is consult-human?

A CLI relay tool that lets AI coding agents (Claude Code, GPT Codex) send questions to a human via messaging apps (Telegram, WhatsApp) and block until the human replies. Designed for developers who run agents with full permissions and want to supervise from their phone.

**How it works:** Agent calls `consult-human ask "question"` → message sent to human on Telegram/WhatsApp → human replies on their phone → CLI unblocks and prints the reply to stdout.

## Build & Development

```bash
# Build
go build -o consult-human .

# Run
go run .

# Test
go test ./...

# Run a single test
go test ./provider -run TestTelegramSend

# Lint (if golangci-lint is installed)
golangci-lint run
```

## Architecture

```
cmd/           CLI commands (cobra). root.go, ask.go, config.go
provider/      Messaging provider interface + implementations (telegram.go, whatsapp.go)
config/        Config loading/saving (~/.config/consult-human/config.yaml)
main.go        Entry point
```

### Key design decisions

- **Provider interface** in `provider/provider.go` defines `Send(ctx, message) → (requestID, error)` and `Receive(ctx, requestID) → (reply, error)`. All messaging backends implement this.
- **stdout is for the reply only.** The `ask` command prints just the raw reply text to stdout so agents can parse it trivially. All status/errors go to stderr.
- **Config** lives at `~/.config/consult-human/config.yaml`. Stores active provider choice and per-provider credentials.
- **No SDK dependencies for Telegram** — uses net/http directly against the Bot API to keep dependencies minimal.

### Adding a new provider

1. Create `provider/yourprovider.go` implementing the `Provider` interface
2. Register it in the provider factory/lookup
3. Add config fields in `config/config.go`

## Skill usage

This tool is designed to be called by AI agents. When hinting agents to use it, add to their instructions:

> When you need human input — a decision, permission, or opinion — run `consult-human ask "your question"`. The reply will be printed to stdout. Do not assume answers to ambiguous questions; consult the human instead.
