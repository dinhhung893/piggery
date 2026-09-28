# Changelog

## v0.2.0 - 2026-09-29

Breaking: a team template with a `tools:` section is now refused. Give its roles `send` and say in
their prompt what to send to whom, then delete the section. An installed built-in template you
edited is not updated for you; edit it the same way.

- Every role of the built-in templates talks with `send` (no `done`/`ask`/`answer` tools).
- Declarative tools are removed: a template with a `tools:` section is refused; use `send`.
- A new session's default name is one word.
- Interactive sessions (pi, Claude, Codex; solos too) show ctx, turns and a tail in `top`, `ps --json` and `piggery tail`, read from the harness's own transcript.

## v0.1.0 - 2026-09-28

First release.

- One Go binary with a local daemon (SQLite, unix socket). Nothing runs in the cloud.
- Mail that waits for an agent to be back, and counts as delivered only when the agent's turn
  that read it has finished.
- Team layouts as YAML: roles, who may talk to whom, who may spawn whom. The daemon checks every
  send and spawn. Built-in: `supervisor-executor`, `slp`, `council`, `p2p`; make your own with
  `piggery template new`.
- Harnesses: pi, Claude Code and Codex (`piggery setup`). Headless workers run in their own
  sessions and can be stopped, resumed and switched to another model.
- Watch and step in: `top`, `ps`, `log`, `tail`, `why`, `abort`, `kill`, `resume`, `model`,
  `release`.
- Paseo plugin: `piggery setup paseo` adds a Piggery view (the same as `piggery top`) to Paseo.
- Upkeep: `gc` archives closed teams, `doctor` checks the daemon's state, `update` installs a
  newer release.
