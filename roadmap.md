# Roadmap

## Completed Milestones

### Phase 1 (Completed): Core CLI + Telegram MVP
- Go stdlib CLI architecture (no Cobra), provider abstraction, and stable ask/reply contract.
- `ask` supports open-ended, multiple-choice, and multiple-choice with `other`.
- Agent-first stdout contract (final answer payload only on stdout).
- Telegram daemonless flow via polling (`getUpdates`) with `/start` chat linking.

### Phase 2 (Completed): Setup, Config, Storage, and Skill UX
- Interactive and non-interactive setup flows.
- Non-interactive Telegram linking via `consult-human setup --provider telegram --link-chat`.
- Shell PATH auto-ensure during setup for login-profile environments.
- `config` and `storage` lifecycle commands (`reset`, `clear`, scoped provider operations).
- Skill install supports global and repo-scoped destinations.
- Skill install injects/updates reminder blocks in `CLAUDE.md`/`AGENTS.md`.

### Phase 3 (Completed): Telegram Reliability Baseline
- File-based pending and inbox coordination for overlapping `ask` processes.
- Per-request expiry, orphan cleanup, and stale-record pruning.
- Multi-pending exact-reply guardrails and reminder behavior.
- Polling hygiene checks (webhook conflict detection, filtered update types).

### Phase 4 (Completed): Distribution and Release Baseline
- GitHub Actions CI (tests, vet, module verification, build checks).
- GitHub release automation on tags with `gh release`.
- Cross-platform release artifacts + checksums.
- Single-command installer script with setup-mode support.

## Prioritized Remaining Work (In Order)

### Phase 5: Telegram Correctness Under Heavy Concurrency
- [ ] Guarantee no-loss/no-misroute behavior under high overlap and rapid reply conditions.
- [ ] Tighten dedupe/idempotency guarantees for retries and duplicate updates.
- [ ] Finalize late-reply policy (expired request replies) and make behavior explicit in code/docs.
- [ ] Add stress-style tests for multi-agent overlap patterns.

### Phase 6: Telegram Formatting + Consultation Context
- [ ] Add provider capability registry in code (formatting, buttons, voice, receipts, runtime model).
- [ ] Add safe Telegram format-aware rendering (`plain`, `telegram_markdown_v2`, `telegram_html`) with escaping.
- [ ] Include stronger session metadata in prompts so humans can disambiguate concurrent agents.
- [ ] Expand structured answer metadata where needed.

### Phase 7: Additional Daemonless Providers
- [ ] Discord provider.
- [ ] Slack provider.
- [ ] Signal provider.
- [ ] Keep email out of scope.

### Phase 8: Relay Mode + WhatsApp Support
- [ ] Implement relay architecture for daemon-based providers:
  relay lifecycle commands (`start|stop|status`), local IPC and locking, and one provider/account relay reused across agent processes.
- [ ] Keep WhatsApp scope to Web session only.
- [ ] Add robust WhatsApp process/session lock and collision handling.
- [ ] Define reconnect/retry behavior and operator guidance.
- [ ] Re-enable WhatsApp after relay + reliability guardrails are in place.

### Phase 9: Voice Messages + Transcription
- [ ] Voice-note intake path in supported channels.
- [ ] Local transcription adapter (Parakeet-first evaluation path).
- [ ] API transcription adapter with clear subscription-vs-API billing behavior.
- [ ] Unified transcript payload shape for agents.
