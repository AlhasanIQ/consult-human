---
name: consult-human
description: Use when an agent needs a human decision, approval, or clarification. This skill explains how to set up and run consult-human for blocking and non-blocking human consultations, including ask flags, setup flows, config reset behavior, and storage/cache clearing commands. Better ask than assume.
compatibility: Designed for Claude Code and Codex CLI with consult-human installed and available on PATH.
---

# consult-human Skill

Use `consult-human` whenever an agent needs a human decision, approval, or clarification.

## Current CLI Surface
- When lost, do `--help`. Its supported universally in the command.
- `consult-human ask [flags] <question>`
- `consult-human setup [flags]`
- `consult-human config <path|show|init|set|reset>`
- `consult-human storage <path|clear>`

## Setup

Run setup before the first consultation or when re-linking Telegram.

Choose setup mode based on who is doing the setup:

1. interactive mode (default): use when the user is driving the setup via their terminal.
2. `--non-interactive` mode: use when the agent will run steps turn-by-turn in via bash commands.

Supported setup flags:
- `--non-interactive`: prints a list of steps to do the set up, without a TTY. Agent-friendly
- `--provider telegram`: restrict setup to Telegram.

### Interactive Setup (User-Driven, TTY)

1. Run `consult-human setup`.
2. Follow Instructions

### Non-Interactive Setup (Agent-Driven, one-shot commands)

Use this mode when you want explicit steps without TTY or terminal interactivity. Agent-friendly.

- `consult-human setup --non-interactive`
- `consult-human setup --non-interactive --provider telegram`

### Reset and Reconfigure

- Reset all config: `consult-human config reset`
- Reset all config but keep storage/cache: `consult-human config reset --keep-storage`
- Reset Telegram only: `consult-human config reset --provider telegram`
- Reset Telegram only but keep storage/cache: `consult-human config reset --provider telegram --keep-storage`
- Re-run Telegram setup: `consult-human setup --provider telegram`
- Explicitly clear storage/cache: `consult-human storage clear`
- Show storage/cache paths: `consult-human storage path`
- Clear Telegram storage/cache only: `consult-human storage clear --provider telegram`
- Show Telegram storage/cache path only: `consult-human storage path --provider telegram`

## The Ask command

All `ask` flags are optional. The only required input is the positional `<question>`.

- `--choice <id:label|label>` (optional, repeatable, default none): adds one selectable option for the human reply. Use `id:label` for stable IDs (example: `A:Ship now`) or plain `label` for auto-generated IDs.
- `--allow-other` (optional, default `false`): allows a free-text answer outside the listed choices, so the human is not forced to pick only from predefined options. Requires at least one `--choice`.
- `--provider <name>` (optional, default is the config field `active_provider`): overrides the active provider used for this ask call. Current active support is Telegram (`whatsapp` is temporarily disabled).
- `--timeout <duration in seconds>` (optional, default configured `request_timeout`): sets how long `ask` waits before timing out for this call. Format examples: `30s`, `5m`, `30m`.

## Blocking Consultation

Use this when execution must pause until the human answers.

1. Run the bash command `consult-human ask "<question>"`.
2. Wait for command completion.
3. Parse stdout JSON as the answer payload.
4. Treat stderr as status/log output only.

Examples:

```bash
consult-human ask "Should I run the migrations now?"
```

```bash
consult-human ask \
  --choice "A:Ship now" \
  --choice "B:Wait for review" \
  --allow-other \
  "Please make a Release decision"
```

If you are Claude Code:
Execute the `consult-human ask ...` command via your Bash tool, setting the `run_in_background` parameter as false, and `timeout` as 600000 (10 min).

## Non-Blocking Consultation

Use this when the agent should continue work while waiting for a human response.

The only way to do async/non-blocking consultation is via leveraging the capabilities in your (the agent's) harness.

If you are Claude Code:
Execute the `consult-human ask ...` command via your Bash tool, setting the `run_in_background` parameter as true, and `timeout` as 600000 (10 min).

If you are Codex:
Due to the constraints of your harness, you cant do this. 

## Practical Notes

- Use `--timeout` for longer waits, for example `--timeout 30m`.
- If a request times out, send a new `ask` request.
- Keep prompts concise and explicit for mobile replies.
- WhatsApp is temporarily disabled; use Telegram for active consultations.



## Commands and Flags Reference

### `ask`

Usage:
- `consult-human ask [flags] <question>`

Flags:
- `--choice <id:label|label>`: Add one choice. Repeatable.
- `--allow-other`: Allow free-text answer outside listed choices. Requires at least one `--choice`.
- `--provider <name>`: Override configured provider for this call.
- `--timeout <duration>`: Override request timeout for this call (`30s`, `5m`, `30m`).

### `setup`

Usage:
- `consult-human setup [--provider telegram]`
- `consult-human setup --non-interactive [--provider telegram]`

Flags:
- `--non-interactive`: Print checklist instead of prompting.
- `--provider <name>`: Restrict setup to a provider (currently `telegram`).

### `config`

Usage:
- `consult-human config path`
- `consult-human config show`
- `consult-human config init`
- `consult-human config set <key> <value>`
- `consult-human config reset [--provider telegram|whatsapp] [--keep-storage]`

Flags:
- `config reset --provider <telegram|whatsapp>`: Reset one provider section only.
- `config reset --keep-storage`: Skip clearing local storage/cache files during reset.

Supported keys for `config set`:
- `default-provider` (aliases: `provider`, `active_provider`)
- `request_timeout`
- `telegram.bot_token`
- `telegram.chat_id`
- `telegram.poll_interval_seconds`
- `telegram.pending_store_path` (alias: `telegram.store_path`)
- `whatsapp.recipient`
- `whatsapp.store_path`

### `storage`

Usage:
- `consult-human storage path`
- `consult-human storage path --provider <all|telegram|whatsapp>`
- `consult-human storage clear`
- `consult-human storage clear --provider <all|telegram|whatsapp>`

Flags:
- `storage path --provider <all|telegram|whatsapp>`: restrict path output scope.
- `storage clear --provider <all|telegram|whatsapp>`: restrict storage clearing scope.
