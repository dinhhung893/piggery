# Changelog

## v0.3.0 - 2026-09-29

Two new harnesses, omp (oh-my-pi) and dsh (DeepSeek Harness 0.2), and each member's current task
in `piggery top`.

Breaking: mail threads, `expects_reply` and the limits `max_hops` and `messages_per_thread` are
removed, and a team template that sets either limit to a number is refused; delete the key. Nothing
in piggery acted on them: `reply_to` stays, and `messages_per_participant_per_minute` is the flood
guard. Mail held by a removed limit is released when the daemon upgrades its database. Teams
already up keep working. Run `piggery setup pi` again so pi sessions stop showing `thread=`.

- omp: `piggery setup omp` adds piggery to omp sessions (mail is steered in and shows in the
  session); a role can run omp workers (`harness: omp`), resumed with their context after a stop.
- dsh 0.2.0-rc.1: `piggery setup dsh` adds piggery to `dsh web` sessions; a role can run dsh
  workers (`harness: dsh`). Workers turn off dsh's upload of session logs to DeepSeek.
- `send --op assign` marks a mail as the member's current task (a spawn or resume task is one);
  `top`'s Overview shows it, whether it was handed back, and a newer unmarked mail.
- `reply_to` takes a bare `N` as well as `#N`.
- The daemon backs up its database before a schema upgrade (`~/.piggery/backups/`, the 3 newest
  kept).
- `~/.piggery` is laid out as `plugins/`, `run/` (per-run scratch), `sessions/` (session data
  piggery keeps); gc removes a removed participant's entries there and in `logs/`, and now also
  removes a solo session gone longer than `gc.closed_after` (archived first, like a closed team).
- `send` answers `sent #N`; a mail's header has no `thread=`; `send --expects-reply` is gone.
- `top`: an open team whose members are all gone is one line; a model switched by `piggery model`
  shows at once. The Paseo plugin starts such a team collapsed (every Paseo app needs 0.9.1+).
- The built-in templates allow 10 live workers at once (`limits.concurrency`, was 4 or 5).
- An installed built-in template file you edited to exactly the new built-in is updated again by
  later versions.
- The gate roles' prompts and the solo card point to `piggery skills` for the rest of piggery.
- One-line install for Linux and macOS:
  `curl -fsSL https://raw.githubusercontent.com/sting8k/piggery/main/install.sh | sh` picks the
  build for your OS and CPU, checks it against the release's `checksums.txt`, and installs it in
  `~/.local/bin` (`PIGGERY_INSTALL_DIR`, `PIGGERY_VERSION` to change).
- A release's notes on GitHub are its section of this changelog.
- Tests that wait for the daemon to start allow 10 seconds, so a slow CI runner no longer fails them.

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
