# Product Vision (Draft)

## The Problem

AI coding agents are increasingly autonomous, but supervision still depends on the developer being physically at a terminal. That creates avoidable stalls and bad assumptions:

- Agents hit ambiguous decisions and guess.
- Developers step away (walk around the house, grab food, smoke break, commute moments) and the agent blocks.
- Terminal-only interaction turns short breaks into lost parallel work time.

Developers want high-autonomy execution and fast human steering, without being desk-bound.

## The Solution

`consult-human` is a CLI bridge between agents and a developer's phone messaging apps.

When an agent needs input, it sends a structured prompt via `consult-human ask`. The human replies from their phone. The CLI returns a machine-parseable answer and the agent continues immediately.

Initial channel focus is Telegram. WhatsApp Web session support is still part of the roadmap, but currently deferred while reliability issues are resolved.

The interaction model supports:

- Open-ended questions
- Multiple-choice questions
- Multiple-choice with an `"other"` free-text path
- Voice-note responses (transcribed) in later phases

## Product Thesis

The best supervision loop is lightweight, mobile, and asynchronous:

- Lightweight enough that agents call it often
- Mobile enough that humans can respond while moving
- Asynchronous enough that neither side is forced into a fragile terminal session

If this works, "away from keyboard" no longer means "agent idle."

## Core Principles

**Agent-first contract.** Stdout emits answer payloads only. Logs/errors go to stderr. No interactive prompts.

**Phone-native UX.** Use messaging apps developers already check constantly. No custom mobile app required.

**Low-ops footprint.** Favor direct provider APIs and local state over always-on backend infrastructure.

**Provider abstraction.** Keep provider adapters thin so channel strategy can evolve without reworking core logic.

**Human clarity over cleverness.** Prompts should be concise, disambiguated, and easy to answer quickly on a lock screen.

## Who This Is For

- Developers running high-autonomy coding agents (Claude Code, Codex, and similar)
- Solo builders who frequently move away from desk but still want tight control
- Teams that need rapid human approvals/choices without halting agent progress

## Critical Use Cases

- Architecture fork: choose one of several implementation paths
- Risky operation confirmation: migrations, deletes, force-pushes, credential changes
- Product choice input: wording, UX options, acceptance criteria
- Unclear requirement clarification: "What should happen in edge case X?"
- On-the-go supervision: respond from phone while not at workstation

## What Success Looks Like

A developer starts a long refactor, leaves the desk, gets a phone message with a multiple-choice decision, taps a response, and returns later to completed work aligned with their intent instead of agent assumptions.
