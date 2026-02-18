# Setup and Configuration

## Setup Modes

Interactive setup (default):

```bash
consult-human setup
```

Checklist output for agent-driven setup:

```bash
consult-human setup --non-interactive
consult-human setup --non-interactive --provider telegram
```

Telegram chat link only (waits for `/start`):

```bash
consult-human setup --provider telegram --link-chat
```

`setup` always ensures the binary path is present in shell login profiles used by agent runtimes.

## Config Commands

```bash
consult-human config init
consult-human config path
consult-human config show
consult-human config set <key> <value>
consult-human config reset
consult-human config reset --provider telegram
consult-human config reset --keep-storage
```

Common keys:

```bash
consult-human config set default-provider telegram
consult-human config set request_timeout 10m
consult-human config set telegram.bot_token "<BOT_TOKEN>"
consult-human config set telegram.chat_id "<CHAT_ID>"              # optional manual override
consult-human config set telegram.pending_store_path "/path/file"
```

## Storage Commands

```bash
consult-human storage path
consult-human storage clear
consult-human storage path --provider telegram
consult-human storage clear --provider telegram
```

## Config Location

Config lookup order:

1. `CONSULT_HUMAN_CONFIG`
2. `$XDG_CONFIG_HOME/consult-human/config.yaml`
3. platform user config dir

## Skill Installation

Global install:

```bash
consult-human skill install --target claude
consult-human skill install --target codex
consult-human skill install --target both
```

Repo-local install:

```bash
consult-human skill install --target claude --repo /path/to/repo
consult-human skill install --target codex --repo /path/to/repo
consult-human skill install --target both --repo /path/to/repo
```

Flags:

- `--target claude|codex|both` (default `both`)
- `--repo <path>` install into a specific repository
- `--source <path>` use a specific local `SKILL.md`
- `--copy` copy file contents (default mode is symlink)
