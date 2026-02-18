# consult-human

`consult-human` lets coding agents ask a human a question through phone messaging apps. We support blocking and non-blocking consultations.
The project is a SKILL.md and a cli tool, that empowers the agent and steers it to ask the human instead of assuming things. 

## Goals

- Agent-first CLI contract: reply payload only on stdout
- Human-friendly mobile loop: answer from Telegram

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
CGO_ENABLED=0 GOFLAGS='-trimpath -mod=readonly -buildvcs=true' \
  go build -ldflags='-s -w -buildid=' -o consult-human .
```

## Install (Single Command)

**Recommended:** Install latest release, then run setup interactively:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/AlhasanIQ/consult-human/main/install.sh)
```

Install latest release, then run setup in non-interactive mode (ie tell your agent to set it up and give it the command):

```bash
curl -fsSL https://raw.githubusercontent.com/AlhasanIQ/consult-human/main/install.sh | bash -s -- --setup-mode non-interactive
```

Pin to a specific release tag:

```bash
curl -fsSL https://raw.githubusercontent.com/AlhasanIQ/consult-human/main/install.sh | bash -s -- --version v0.1.0 --setup-mode non-interactive
```

Installer notes:

- downloads the right release asset for your OS/arch from GitHub Releases
- verifies SHA256 using `checksums.txt`
- installs `consult-human` to `~/.local/bin` by default (override with `--install-dir`)
- supports `--setup-mode auto|interactive|non-interactive|skip`

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
consult-human config reset --keep-storage

# explicit storage/cache clear
consult-human storage clear
consult-human storage path
consult-human storage clear --provider telegram
consult-human storage path --provider telegram
consult-human storage clear --provider whatsapp
consult-human storage path --provider whatsapp

# interactive first-time setup
consult-human setup
# includes required final skill-install target selection (claude/codex/both)
# and required install-scope selection (global/current repo/custom repo path)
# and auto-ensures consult-human binary directory is on shell PATH

# checklist-only setup plan (non-interactive)
consult-human setup --non-interactive
consult-human setup --non-interactive --provider telegram
# non-interactive mode also auto-ensures shell PATH (no extra prompt)
# non-interactive chat linking via /start (no prompts)
consult-human setup --provider telegram --link-chat

# install SKILL.md into agent runtimes
consult-human skill install --target claude
consult-human skill install --target codex
consult-human skill install --target both
# install into a specific repo (project-local skills)
consult-human skill install --target claude --repo /path/to/repo
consult-human skill install --target codex --repo /path/to/repo
# alias:
consult-human install-skill --target claude
```

`config reset` now clears local storage/cache by default.
Use `--keep-storage` if you only want to reset config keys.

`skill install` defaults:

- source path defaults to `<config-dir>/SKILL.md` where `<config-dir>` is the directory of `consult-human config path`
- managed source is refreshed from the binary’s embedded skill template when it changes
- install mode defaults to symlink (`--link=true`)
- `consult-human storage clear` (all scope) and `consult-human config reset` delete this managed skill file
- also appends/updates an IMPORTANT consult-human reminder block in agent instruction files:
  - Claude: `<base>/.claude/CLAUDE.md`
  - Codex: `<base>/.codex/AGENTS.md`
  - Agents (when present): `<base>/.agents/AGENTS.md`
  - where `<base>` is home for global install, or `--repo` path for repo-scoped install

Interactive `setup` behavior for skill install:

- setup shows the concrete destination path(s) for each install scope
- global scope installs for all Claude/Codex sessions in this user account on this machine
- if you are inside a git repo and it has `.claude` or `.agents`, setup defaults scope to current repo
- setup also offers custom repo path input

`setup` PATH behavior (interactive and non-interactive):

- auto-detects shell (`zsh` or `bash`) and upserts PATH in a login profile
- `zsh`: prefers existing `~/.zshenv`, then `~/.zprofile`, then `~/.zlogin` (fallback creates `~/.zshenv`)
- `bash`: prefers existing `~/.bash_profile`, then `~/.bash_login` (fallback creates `~/.bash_login`)
- prints what file/path was used; no extra command required
- this is important for Claude Code VS Code extension environments where `~/.zshrc`/`~/.bashrc` may not be loaded

Telegram non-interactive chat linking:

- `consult-human setup --provider telegram --link-chat` waits for `/start`, captures `telegram.chat_id`, and saves config without interactive prompts

`skill install` flags:

- `--target claude|codex|both` (default: `both`)
- `--repo <path>` to install inside a specific repository instead of user-global directories
- `--source <path>` to install from a specific local `SKILL.md`
- `--copy` to copy file contents instead of symlinking

Set provider and credentials:

```bash
# Telegram
consult-human config set default-provider telegram
consult-human config set telegram.bot_token "<BOT_TOKEN>"
consult-human config set telegram.pending_store_path "~/.local/state/consult-human/telegram-pending.json"
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

## Agent Skill

- See `SKILL.md` for agent-facing instructions.
- Primary supported runtime is Claude Code.

## Codex Status (Currently Unsupported)

`consult-human` is currently not supported for Codex-based runtime flows.

Why:

- In our Codex harness tests, long-running tool calls return control quickly and require continued polling of a live session ID to observe completion. If you send any command in steer-mode, codex will abandon polling. And even if you just treat it as a blocking consultation, codex will stop polling soon after. 
- `consult-human ask` may still complete in the background, but if the agent turn is no longer polling, the human reply is not delivered back into the active agent flow.
- Workarounds like App Server/MCP relay are not aligned with the goals of this project.

## WhatsApp Status

- WhatsApp provider is temporarily disabled due stability issues.
- Existing WhatsApp config/state can remain on disk, but `ask` only supports Telegram for now.

## Telegram Notes

- `telegram.chat_id` is optional. The setup command can fetch it automatically after receiving the first `/start` message sent from you to the bot.
- On first `/start`, the tool captures and saves that chat ID automatically.
- Telegram supports `parse_mode` formatting (`MarkdownV2`/`HTML`) in Bot API; current default output remains plain text until provider-format metadata/escaping is wired in.
- Prompts are sent with Telegram `ForceReply` markup so clients open a direct reply UI.
- `ask` verifies polling mode by checking `getWebhookInfo`; if a webhook URL is set, it fails fast with an actionable error.
- Polling calls use `allowed_updates=["message"]` to reduce non-message noise in this ask/reply flow.
- Pending Telegram requests are stored on disk so multiple command instances can coordinate.
- A shared Telegram inbox file plus a single-poller lock coordinates `getUpdates` calls across overlapping `ask` processes.
- Coordination only applies to instances sharing the same Telegram store directory (same machine/path by default).
- If agents run on different machines, they will not coordinate unless you centralize that store directory (for example, using a network file path).
- Pending records now carry an expiry per request, derived from that request's own timeout (`ask --timeout` or `request_timeout`), so instances with different timeout values are handled correctly.
- Expired/abandoned pending records are pruned automatically during store operations.
- Pending records also carry owner PID/host metadata; if a local process dies unexpectedly, orphaned records are pruned without waiting for full timeout expiry.
- Telegram inbox entries are also pruned automatically: ambiguous free-text is dropped immediately in multi-pending mode, and unclaimed entries expire on TTL.
- With one active request, plain text in the chat is still accepted as a reply (backward-compatible behavior).
- In that single-active fallback mode, only messages newer than the original prompt are accepted.
- If multiple unanswered consult-human requests are active, replies must be direct message replies (`Reply` on the exact bot message).
- In that multi-active case, a non-reply message triggers a reminder to reply to the exact message.
- Default pending store path: `${XDG_STATE_HOME:-~/.local/state}/consult-human/telegram-pending.json`
- Default inbox store path: `${XDG_STATE_HOME:-~/.local/state}/consult-human/telegram-inbox.json`
- Override in config with `telegram.pending_store_path` (alias `telegram.store_path`).
- Environment override still takes precedence: `CONSULT_HUMAN_TELEGRAM_PENDING_STORE=/custom/path/telegram-pending.json`
- Run `consult-human storage path --provider telegram` to print the effective pending/inbox paths.

Telegram best-practice sources:
- Bot API (`getUpdates`, `getWebhookInfo`, `ForceReply`, formatting): https://core.telegram.org/bots/api
- Bot FAQ (`getUpdates` offset usage and polling guidance): https://core.telegram.org/bots/faq#how-do-i-get-updates

## Test

```bash
go test ./...
```

## CI/CD

- CI workflow (`.github/workflows/ci.yml`) runs on push to `main` and pull requests:
  - `go test ./...`
  - `go mod verify`
  - `go vet ./...`
  - `CGO_ENABLED=0 GOFLAGS='-trimpath -mod=readonly -buildvcs=true' go build -ldflags='-s -w -buildid=' -o consult-human .`
  - `bash -n install.sh`
- Release workflow (`.github/workflows/release.yml`) runs on tag pushes like `v0.1.0`:
  - runs tests
  - verifies modules and vets code
  - cross-builds static release binaries for `linux/darwin` and `amd64/arm64`
  - creates deterministic tar archives
  - generates `checksums.txt`
- publishes release assets with `gh release create`/`gh release upload`

Note: prerelease tags like `v0.1.0-rc.1` are currently published as normal releases, so installer `latest` may resolve to them.

Release trigger:

```bash
git tag v0.1.0
git push origin v0.1.0
```
