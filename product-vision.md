# Product Vision

## The Problem

AI coding agents (Claude Code, GPT Codex) are increasingly capable of working autonomously on complex tasks. Developers grant them full permissions and let them run — but this creates a supervision gap:

- **Agents assume answers** when they hit ambiguous decisions, leading to wasted work or wrong directions.
- **Terminal-bound interaction** means the developer must sit at their computer to answer agent questions.
- **Permission prompts block** the agent entirely if the developer steps away.

Developers want the productivity of autonomous agents with the safety of human oversight — without being chained to their desk.

## The Solution

**consult-human** is a CLI relay between AI agents and humans via messaging apps.

When an agent needs input, it runs `consult-human ask "Should I refactor this into a separate service?"`. The question appears on the developer's phone (Telegram, WhatsApp). They reply. The agent unblocks and continues.

## Core Principles

**Agent-first design.** The CLI output, flags, and behavior are optimized for consumption by AI agents, not humans. Stdout carries only the reply. Errors go to stderr. No interactive prompts.

**Messaging-app native.** Meet developers where they already are. Telegram and WhatsApp first — apps people already have open. No custom app to install.

**Zero infrastructure.** No server to deploy. The CLI talks directly to messaging APIs. Config is a local YAML file. Install with `go install` or brew.

**Provider agnostic.** A clean provider interface means adding Slack, Discord, or email is a matter of implementing two methods: `Send` and `Receive`.

## Who Is This For

- Developers running Claude Code with `--dangerously-skip-permissions` or similar full-access modes
- Teams using GPT Codex or other autonomous coding agents
- Anyone who wants to supervise AI agents from their phone while away from their desk

## What Success Looks Like

A developer kicks off a complex refactoring task with Claude Code, walks to get coffee, and gets a Telegram message: *"The auth module has 3 circular dependencies. Should I (a) break them by extracting a shared types package, or (b) inline the shared code? Reply a or b."* They reply "a", and by the time they're back, the refactor is done — correctly.
