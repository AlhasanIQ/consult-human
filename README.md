# consult-human

`consult-human` lets coding agents ask a human a question through phone messaging apps, then block until a reply is received.

## Goals

- Agent-first CLI contract: reply payload only on stdout
- Human-friendly mobile loop: answer from Telegram
- No command framework dependency: stdlib CLI parsing

## Current Scope

- `ask` command with:
  - open-ended questions
  - multiple-choice questions
  - multiple-choice with `other` free text
- `config` command for local YAML config
- Providers:
  - Telegram Bot API
  - WhatsApp is temporarily disabled (deferred to later roadmap phase)

## Build

```bash
go build -o consult-human .
```

## Configuration

Config path resolution:

1. `CONSULT_HUMAN_CONFIG` if set
2. `$XDG_CONFIG_HOME/consult-human/config.yaml`
3. `~/.config/consult-human/config.yaml` (platform equivalent via `os.UserConfigDir`)

Initialize and inspect config:

```bash
consult-human config init
consult-human config path
consult-human config show
consult-human config reset
consult-human config reset --provider telegram
consult-human config reset --provider whatsapp

# interactive first-time setup
consult-human setup

# checklist-only setup plan (non-interactive)
consult-human setup --non-interactive
consult-human setup --non-interactive --provider telegram
```

Set provider and credentials:

```bash
# Telegram
consult-human config set default-provider telegram
consult-human config set telegram.bot_token "<BOT_TOKEN>"
# Optional manual override:
# consult-human config set telegram.chat_id "<CHAT_ID>"

# WhatsApp is temporarily disabled
```

## Usage

Open-ended:

```bash
consult-human ask "Should I split this package now or defer?"
```

Multiple choice:

```bash
consult-human ask \
  --choice "A:Extract shared package" \
  --choice "B:Inline duplicated code" \
  "Auth module has circular deps. Which option?"
```

Multiple choice with `other`:

```bash
consult-human ask \
  --choice "A:Proceed" \
  --choice "B:Stop" \
  --allow-other \
  "Deploy now?"
```

Example stdout payload:

```json
{"request_id":"2d4a4f4f5a6207bb","provider":"telegram","question_type":"choice","text":"B","selected_ids":["B"],"raw_reply":"B","received_at":"2026-02-18T05:00:00Z"}
```

All status logs and errors are written to stderr.

## WhatsApp Status

- WhatsApp provider is temporarily disabled due stability issues.
- Existing WhatsApp config/state can remain on disk, but `ask` only supports Telegram for now.

## Telegram Notes

- `telegram.chat_id` is optional.
- If it is not set, `consult-human ask` waits for a `/start` message sent to your bot.
- On first `/start`, the tool captures and saves that chat ID automatically.
- Pending Telegram requests are stored on disk so multiple command instances can coordinate.
- Coordination only applies to instances sharing the same pending-store file (same machine/path by default).
- If agents run on different machines, they will not coordinate unless you centralize that store.
- With one active request, plain text in the chat is still accepted as a reply (backward-compatible behavior).
- If multiple unanswered consult-human requests are active, replies must be direct message replies (`Reply` on the exact bot message).
- In that multi-active case, a non-reply message triggers a reminder to reply to the exact message.
- Default pending store path: `${XDG_STATE_HOME:-~/.local/state}/consult-human/telegram-pending.json`
- Override with `CONSULT_HUMAN_TELEGRAM_PENDING_STORE=/custom/path/telegram-pending.json`

## Test

```bash
go test ./...
```
