# Roadmap (Draft)

## Product Constraints (Current Direction)

- [x] CLI uses Go standard library only (no Cobra)
- [x] Provider-agnostic core with small adapter surface
- [x] Agent-first UX: stdout contains only the final answer payload
- [x] Mobile-first UX: optimized for supervision while away from desk
- [x] WhatsApp design target is Web session only (no Cloud API / Twilio / hosted relay)

## Phase 0: Foundations and Decisions

- [x] Command architecture with `flag` + explicit subcommand dispatch
- [x] Decide config path strategy:
  - XDG default (`$XDG_CONFIG_HOME/consult-human/config.yaml` or platform equivalent)
  - `CONSULT_HUMAN_CONFIG` override
  - optional local-project config mode for repo-scoped setup (deferred)
- [x] Freeze message envelope schema (request ID, timestamps, question type, metadata)
- [x] Define provider capability matrix (text, buttons, voice, delivery/seen receipts, formatting)

## Provider Capability Matrix (Research Snapshot: 2026-02-18)

| Provider | Text | Choice UX | Voice Inbound | Delivery/Seen | Formatting | Runtime Profile | Fit for This Project |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Telegram Bot API | Yes | Inline keyboards + callback queries | Yes (`Message.voice`) | No true read receipts in Bot API (delivery ack only) | `parse_mode` supports `MarkdownV2` and `HTML` | Daemonless-safe via `getUpdates` polling | Primary/default now |
| WhatsApp Web (whatsmeow, currently disabled) | Yes | Text-first (interactive UX can be layered later) | Yes (media messages) | Library supports receipt primitives | No explicit parse mode; formatting is message-text conventions | Session/WebSocket + persistent device store | Re-enable only after reliability guardrails |
| Signal (`signal-cli`) | Yes | Text-first (no native button API) | Yes (attachments) | Receipt commands are supported | Plain text (no markdown mode contract) | Requires daemon / regular receive polling | Good mobile UX, but unofficial/ops-heavy |
| Discord Bot API | Yes | Message components (buttons/select menus) | Yes (voice messages are attachment-based) | No read/seen API for bot DMs/channels | Discord markdown formatting supported in message content | Gateway WebSocket + intents for content access | Strong candidate, relay-recommended |
| Slack App API | Yes | Block Kit buttons/select + text input elements | Audio can be handled as file events/uploads | `im_marked` exists but read semantics are limited | `mrkdwn` formatting supported | Events endpoint (public URL) or Socket Mode WebSocket | Team-centric candidate, relay-recommended |
| iMessage bridge (BlueBubbles/others) | Bridge-dependent | Bridge-dependent | Bridge-dependent | Bridge-dependent | Bridge-dependent | Requires always-on Mac relay; private API modes may require SIP changes | Low priority / high ops risk |

Provider expansion decision (current):
- `daemonless-safe`: Telegram.
- `relay-recommended`: Discord, Slack, Signal, WhatsApp Web.
- `defer`: iMessage bridge paths unless there is strong user demand.

Formatting decision (new):
- Add provider formatting capability metadata to the internal model so prompt rendering can target channel-native syntax safely (`plain`, `telegram_markdown_v2`, `telegram_html`, `slack_mrkdwn`, `discord_markdown`).
- Default to `plain` unless provider capability + escaping rules are explicitly enabled.

Research sources:
- Telegram Bot API: https://core.telegram.org/bots/api
- Telegram Bot FAQ (`getUpdates`/offset guidance): https://core.telegram.org/bots/faq#how-do-i-get-updates
- Whatsmeow (features + scope): https://github.com/tulir/whatsmeow
- Whatsmeow SQL store: https://pkg.go.dev/go.mau.fi/whatsmeow/store/sqlstore
- Signal CLI: https://github.com/AsamK/signal-cli
- Discord Gateway: https://docs.discord.com/developers/docs/topics/gateway
- Discord message components: https://docs.discord.com/developers/docs/interactions/message-components
- Discord message object (voice message flag): https://docs.discord.com/developers/resources/message
- Slack Events API: https://api.slack.com/apis/connections/events-api
- Slack Socket Mode: https://api.slack.com/apis/connections/socket
- Slack Block Kit text/inputs: https://docs.slack.dev/reference/block-kit/
- Slack `im_marked` event: https://api.slack.com/events/im_marked
- Slack `mrkdwn`: https://docs.slack.dev/messaging/formatting-message-text/
- Discord markdown formatting reference: https://support.discord.com/hc/en-us/articles/210298617
- BlueBubbles server requirements: https://docs.bluebubbles.app/server/
- BlueBubbles private API caveats: https://docs.bluebubbles.app/private-api/installation

## Phase 1: MVP (Blocking Ask Loop)

- [x] Provider interface in `provider/provider.go`:
  - `Send(ctx, request) -> (requestID, error)`
  - `Receive(ctx, requestID) -> (reply, error)`
- [x] Telegram provider (Bot API; polling first, webhook later)
- [ ] WhatsApp provider re-enable (currently disabled; deferred)
- [x] `ask` command v1:
  - open-ended question
  - multiple-choice question
  - multiple-choice with `"other"` free-text path
  - print answer payload to stdout only
- [x] `config` command v1 (set provider, tokens, recipient identity)
- [x] request timeout + cancellation semantics
- [x] README quickstart + agent usage examples

## WhatsApp Web Session Track (Deferred)

- [x] Prototype implemented with local Web-session transport only
  - QR link flow in CLI
  - persistent local auth/session state
- [x] Deferred from active provider list due session reliability/stability issues
- [ ] Re-enable with explicit guardrails
  - single-session/process lock and better collision errors
  - documented reconnect behavior and safe retry strategy
  - clear operator guidance when a background relay is required
- [ ] Keep transport boundary clean so future provider additions do not require core refactor

## Background Process: Relay vs. Pure CLI (Architectural Decision)

Not all providers are equal — some are stateless HTTP APIs, others require persistent
connections. This has a direct impact on whether the tool can stay a simple CLI binary
or needs a background relay process.

**Daemonless-safe today:**
- Telegram Bot API (`getUpdates` polling).
- Each `consult-human ask` invocation can stay self-contained: send request, poll for
  reply, print answer, exit.

**Relay-recommended (or hosted endpoint required):**
- Slack App API (Events API request URL or Socket Mode WebSocket)
- Discord Bot API (Gateway WebSocket + intents for inbound message content)
- These typically need an always-on local relay or hosted callback endpoint for reliable inbound handling.

**Session-based providers (persistent connection required):**
- WhatsApp Web (WebSocket session — drops if nothing keeps it alive)
- Signal (registration + persistent receive connection)
- These can work in a foreground blocking ask loop, but usually hit limits sooner
  for multi-agent concurrency, async flows, and long-lived reliability without a relay.

**Implementation reality check (current code):**
- [x] `consult-human ask` itself is a long-running foreground process while waiting for reply
- [x] Telegram works well in daemonless mode (HTTP polling + shared pending-request file)
- [x] WhatsApp implementation exists but is currently disabled in provider factory/config selection
- [ ] Revisit WhatsApp daemonless viability after concurrency guardrails and session-lock handling

**Decided approach (updated):**
- [x] Keep daemonless UX as the default install path (critical for skill adoption)
- [x] Telegram remains recommended default for zero-ops setup
- [ ] Add provider-level concurrency guardrails for session transports (initially WhatsApp):
  - fail fast with explicit error when concurrent session clients are detected
  - guide user toward relay mode for multi-agent usage
- [ ] Re-enable WhatsApp only after those guardrails are in place
- [ ] Introduce relay mode only when required by concrete triggers:
  - multiple concurrent asks on session-based providers
  - background ask workflows where the runtime may outlive or detach from individual sessions
  - stronger reliability/SLO requirements (auto-reconnect, queueing, crash recovery)
- [ ] Relay architecture (when triggered):
  - `consult-human relay start|stop|status`
  - local IPC (Unix socket / named pipe) + pidfile/lockfile
  - one relay per provider/account, reused by all agent processes
- [ ] Optional auto-start behavior (post-relay):
  - `ask` attempts relay socket; if missing, starts relay on demand
  - relay idle timeout to avoid permanent background daemons by default
- [ ] Documentation requirement:
  - clearly label each provider as `daemonless-safe` vs `relay-recommended`
  - keep skill instructions simple: agents call `consult-human ask`; relay is
    a one-time local user setup only when needed

## Agent Runtime Research: Claude Code Background + Async/Poll Patterns

Research snapshot (2026-02-18):
- Claude Code supports background execution in interactive mode for long-running commands.
  - Commands can be moved to background (`Ctrl+B`) and continue while the session accepts new prompts.
  - Background tasks have IDs and buffered output retrieval via `TaskOutput`.
  - Background tasks are cleaned up when Claude Code exits.
- Claude Code supports non-interactive/automation mode via `claude -p`.
  - Structured outputs: `--output-format json` or `--output-format stream-json`.
  - Streaming mode supports partial events with `--include-partial-messages --verbose`.
  - Conversation continuity for automation: `--continue` and `--resume`.
  - Bidirectional streaming input is supported with `--input-format stream-json` in print mode.
- Claude Code hooks support background execution with `async: true` (command hooks only).
  - Async hooks do not block/deny actions and cannot enforce decisions.
  - Async hook output is injected on the next conversation turn (not immediate if idle).

Implications for `consult-human`:
- Keep blocking `ask` as the reliable default for decision gates.
- Do not add async CLI primitives (`send`/`poll`/`wait`) at this stage.
- Document async usage as an agent-runtime pattern: run `consult-human ask ...` in background using native agent tooling.
- Prefer native task output retrieval from the agent runtime over introducing a second async protocol in this CLI.
- Treat Claude async hooks as best-effort notification plumbing, not control-flow enforcement.

Source links:
- Claude Code interactive mode (background tasks): https://code.claude.com/docs/en/interactive-mode
- Claude Code hooks (async hooks): https://code.claude.com/docs/en/hooks
- Claude Code programmatic usage (`-p`, json/stream-json, continue/resume): https://code.claude.com/docs/en/headless
- Claude Code CLI reference (`--input-format`, `--output-format`, `--continue`, `--resume`): https://code.claude.com/docs/en/cli-usage

## Phase 2: Agent Ergonomics and Reliability

- [x] Async usage pattern documented via agent-runtime background execution (`ask` only)
- [x] Local pending-request store (file-based) for Telegram cross-process coordination
- [x] Telegram pending-request expiry model (per-request timeout-derived `expires_at`) + auto-pruning of abandoned records
- [x] Telegram orphan cleanup model:
  - owner PID/host metadata on pending records
  - dead local owner processes are pruned early (no need to wait full timeout)
- [x] Signal-aware ask cancellation path (SIGINT/SIGTERM) to reduce stale pending entries
- [x] Telegram threading guardrails:
  - ForceReply prompt markup
  - exact-reply required when multiple pending questions exist
  - single-pending fallback only accepts messages newer than the original prompt
- [x] Telegram polling hygiene:
  - fail fast if webhook mode is active (`getWebhookInfo` must be empty URL)
  - poll only `message` updates (`allowed_updates=["message"]`)
- [x] Local storage management commands:
  - `consult-human storage clear` for explicit cache/state cleanup
  - `config reset` clears storage by default (`--keep-storage` opt-out)
- [ ] Generalize pending store strategy across providers where applicable
- [ ] Telegram multi-process polling hardening:
  - current inference from Bot API semantics: concurrent `getUpdates` consumers can still race on offsets
  - add shared update cursor/inbox (or relay mode) for strict no-loss behavior under heavy concurrency
- [ ] Idempotency and dedupe (retries must not create duplicate human prompts)
- [ ] Retry and backoff policy per provider
- [ ] Strong correlation for concurrent asks from multiple agent sessions
- [ ] Provider capability registry in code (buttons/voice/receipts/formatting/runtime model)
- [ ] Prompt render strategy per provider format with safe escaping
- [ ] Session context in messages: include enough thread/session info (e.g. project name,
  agent identity, task summary) so the human can tell which agent session a question
  came from — critical when multiple agents are running concurrently
- [ ] Structured answer payload for agents:
  - question type
  - selected option ID(s) when applicable
  - free-text value when applicable
  - formatting mode used when message was sent
  - timestamps + source metadata

## Phase 3: Providers for Phone-First Developer UX

- [ ] Signal provider (strong mobile UX + privacy)
- [ ] Discord provider (widely used by developer communities, good mobile app)
- [ ] Slack provider (workplace-heavy usage, practical for teams)
- [ ] iMessage path (Apple-only, likely via bridge tooling)
- [ ] Keep email out of roadmap

## Phase 4: Voice Messages + Transcription

- [ ] Voice-note ingestion in supported channels (starting with WhatsApp/Telegram)
- [ ] Local transcription adapter:
  - evaluate Parakeet as primary local engine
  - document CPU/GPU runtime expectations
- [ ] API transcription adapter:
  - prefer subscription-backed model access if viable in Claude/Codex ecosystems
  - verify what is and is not covered by subscription plans vs API billing
  - graceful fallback to BYO API key
- [ ] Unified transcript payload shape for agents

## Phase 5: Distribution and Ecosystem

- [ ] `go install` support (`go install github.com/AlhasanIQ/consult-human@latest`)
- [ ] Homebrew formula
- [ ] Prebuilt binaries (goreleaser)
- [ ] Optional MCP server mode
- [ ] Published skill snippets for Codex/Claude agent instructions

## Agent-Centric Edge Cases (Must Handle Early)

- [ ] Human responds after timeout (late reply policy: ignore, map to expired request, or reopen)
- [ ] Human replies in wrong thread/chat (routing ambiguity)
- [ ] Multiple agents share one human inbox (identity + request scoping)
- [ ] Partial/ambiguous multiple-choice replies ("2", "B", typo, mixed text)
- [ ] Human sends media/voice when text expected
- [ ] Provider outage during block/wait
- [ ] Duplicate inbound webhook/update events
- [ ] Agent process restarts while waiting
- [ ] Human asks follow-up question instead of answering
- [ ] Security: verify sender identity and guard against unsolicited inbound messages
