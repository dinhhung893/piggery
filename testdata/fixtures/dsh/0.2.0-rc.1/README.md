dsh 0.2.0-rc.1 (tag dsh-v0.2.0-rc.1) captures, sdk profile `session.event`/`session.status` notifications of a real run
(example/glm-5.3-flash as a custom llm-pi-ai provider). `events-<scenario>.jsonl`: basic (prompt, then mail into an idle agent),
stopping (mail steered at agent/turn-stopping: a second step in the same turn), midturn (steer during three tool calls),
abort (agent.cancel with keepInbox during a tool call), tool (a plugin tool called by the model).
Trimmed: system prompt, skill catalog, runtime context and tool descriptions replaced by "[trimmed]"; the assistant `stream` and
`replayState` dropped; the probe plugin's mail source kind `piggery-probe` renamed `piggery-mail`. The other files here (drive.mjs,
probe plugin, raw captures) are research leftovers and not committed.
