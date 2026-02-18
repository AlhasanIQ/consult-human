# Runtime Compatibility

## Claude Code

Supported target.

Recommended pattern:

1. Ensure `consult-human` is on `PATH` (`consult-human setup` does this).
2. Run `consult-human ask ...` from Bash tool.
3. Parse JSON result from `stdout`.

For longer waits, increase command timeout in your runtime so it can outlive the consultation window.

## Codex

Currently not a supported target for reliable blocking/non-blocking consultation.

Reason: runtime command-session behavior can end polling/streaming before a delayed human reply arrives, so the agent may miss the completion event.

Use Claude Code as the primary runtime until Codex command-session behavior is reliable enough for this flow.
