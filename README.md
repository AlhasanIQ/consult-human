# consult-human

`consult-human` adds human-in-the-loop for coding agents. It lets an agent send a question to your phone (currently Telegram), then act upon for your reply (blocking/non-blocking).
The CLI tool handles setup and messaging app wiring, the SKILL.md teaches the agent how and when to use, and the CLAUDE.md reminder reminds the agents to use the skill.

## Why Use It

- Lets you keep using agents with `--dangerously-skip-permissions` without them going off-rail and doing crazy things
- Prevents risky assumptions by agents.
- Keeps approvals and decisions mobile-friendly and on-the-go.
- The only way to do human-in-the-loop in both blocking and non-blocking manners on the go.

## Current Status

### Providers

| Provider | Support | Notes |
| --- | --- | --- |
| Telegram | ✅ Supported | Active provider. |
| WhatsApp | ❌ Not Supported (in roadmap) | Temporarily disabled (planned for a later phase). |

### Agent Runtimes

| Runtime | Support | Notes |
| --- | --- | --- |
| Claude Code | ✅ Supported | Blocking and non-blocking flows are supported. |
| Codex | ❌ Not Supported (for now) | Codex'es Harness foreground/background execution limitations. |


## Install

Interactive install + setup (Recommended):

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/AlhasanIQ/consult-human/main/install.sh)
```

Agent-driven setup checklist:

```bash
curl -fsSL https://raw.githubusercontent.com/AlhasanIQ/consult-human/main/install.sh | bash -s -- --setup-mode non-interactive
```

## Quick Start

```bash
consult-human setup
consult-human ask "Ship this change now?"
```

Example `stdout` payload:

```json
{"request_id":"...","provider":"telegram","question_type":"open","text":"Sure you can ship","raw_reply":"Sure you can ship","received_at":"..."}
```

## Non-Intuitive Gotchas

- `setup` auto-adds the binary path to login profiles (`~/.zshenv`, `~/.zprofile`, `~/.zlogin`, `~/.bash_profile`, `~/.bash_login` depending on your shell) so agent shells can find `consult-human`. We can't use `~/.zshrc`/`~/.bashrc` due to behavioral discrepency between how claude code loads shell profiles in cli and in VS Code extensions.
- Codex runtime is currently not a supported target for reliable blocking/non-blocking waits.

## Docs

- Setup and config: `docs/setup-and-config.md`
- Telegram behavior and edge cases: `docs/telegram.md`
- Runtime compatibility (Claude/Codex): `docs/runtime-compat.md`
- Release and distribution notes: `docs/release.md`
- Agent skill instructions: `SKILL.md`
- Product roadmap: `roadmap.md`

## License

MIT. See `LICENSE`.
