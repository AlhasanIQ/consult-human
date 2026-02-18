# Telegram Provider Notes

## What It Uses

- Telegram Bot API over HTTPS (`net/http`).
- Long polling via `getUpdates` (no webhook mode).

## Setup Requirements

1. Create a bot in `@BotFather` and set `telegram.bot_token`.
2. Link your chat by sending `/start` to your bot (`consult-human setup --provider telegram` or `--link-chat`).

## Reply Matching Rules

- If one question is pending, a normal text message after the prompt can be accepted.
- If multiple questions are pending in the same chat, replies must be threaded (reply to the exact message).
- Ambiguous non-threaded replies are dropped and a reminder is sent to reply to the exact message.

## Multi-Process Behavior

- Pending requests and inbox updates are stored on disk.
- A poller lock allows only one active Telegram poller per shared store path.
- Multiple `consult-human ask` processes on the same machine/path coordinate through these files.

If different machines use different store paths, they do not share pending state.

## Storage Files

Use:

```bash
consult-human storage path --provider telegram
```

This reports the pending-store JSON and inbox-store JSON paths.

## Cleanup Behavior

- Pending requests expire automatically (deadline-based where available; legacy fallback TTL applies).
- Inbox entries are TTL-pruned to avoid stale buildup.
- Manual cleanup: `consult-human storage clear --provider telegram`

## Common Failure Cases

- `telegram webhook is configured`: disable webhook for that bot token.
- `chat is not linked`: send `/start` to the bot, then retry.
