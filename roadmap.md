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
- [ ] Define provider capability matrix (text, buttons, voice, delivery/seen receipts)

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

**Stateless providers (no background process needed):**
- Telegram Bot API, Slack Bot API, Discord Bot API
- Each `consult-human ask` invocation is self-contained: send HTTP request, poll for
  reply, print answer, exit. Nothing runs between invocations.

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
  - async workflows (`send` now, `wait` later) where ask process is not kept alive
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

## Phase 2: Agent Ergonomics and Reliability

- [ ] Async pattern (if needed): `send` / `poll` / `wait`
- [x] Local pending-request store (file-based) for Telegram cross-process coordination
- [ ] Generalize pending store strategy across providers where applicable
- [ ] Idempotency and dedupe (retries must not create duplicate human prompts)
- [ ] Retry and backoff policy per provider
- [ ] Strong correlation for concurrent asks from multiple agent sessions
- [ ] Session context in messages: include enough thread/session info (e.g. project name,
  agent identity, task summary) so the human can tell which agent session a question
  came from — critical when multiple agents are running concurrently
- [ ] Structured answer payload for agents:
  - question type
  - selected option ID(s) when applicable
  - free-text value when applicable
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
