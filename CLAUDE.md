# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is consult-human?

A CLI relay tool that lets AI coding agents (Claude Code, GPT Codex) send questions to a human via messaging apps and block until the human replies. Designed for developers who run agents with full permissions and want to supervise from their phone while away from desk.

**How it works:** Agent calls `consult-human ask ...` → message sent to human on a provider channel → human replies on phone → CLI unblocks and prints the answer payload to stdout.

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
cmd/           CLI command parsing/dispatch (stdlib-based)
provider/      Messaging provider interface + implementations
config/        Config loading/saving (XDG + env override)
main.go        Entry point
```

### Key design decisions

- **Provider interface** in `provider/provider.go` defines `Send(ctx, request) → (requestID, error)` and `Receive(ctx, requestID) → (reply, error)`. All messaging backends implement this.
- **stdout is for the answer payload only.** The `ask` command prints the machine-consumable answer to stdout. All status/errors go to stderr.
- **Config path is configurable.** Uses XDG-compatible defaults with `CONSULT_HUMAN_CONFIG` override.
- **Question modes** include open-ended and multiple-choice (including `other` free-text replies).
- **WhatsApp transport direction is Web-session based, but currently disabled.** No Cloud API/Twilio path in current scope.
- **Keep dependencies minimal** where practical; use direct HTTP/API integrations when it improves maintainability.

### Adding a new provider

1. Create `provider/yourprovider.go` implementing the `Provider` interface
2. Register it in the provider factory/lookup
3. Add config fields in `config/config.go`

## Skill usage

This tool is designed to be called by AI agents. When hinting agents to use it, add to their instructions:

> When you need human input — a decision, permission, or opinion — run `consult-human ask "your question"`. The reply will be printed to stdout. Do not assume answers to ambiguous questions; consult the human instead.
