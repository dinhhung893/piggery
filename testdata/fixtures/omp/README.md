# omp 18.4.2 captures (headless worker side)

Real runs of `omp --mode rpc` (oh-my-pi v18.4.2, darwin-arm64) with the model `example/glm-5.3-flash`, under a
throwaway `HOME`, no piggery daemon. A `.jsonl` file is what omp wrote on stdout, one JSON frame per line, as
received. Only these things differ from the raw capture: `available_commands_update.commands` is cut to its
first 2 entries, `get_state.data.systemPrompt` to 160 chars and `dumpTools` to the tool names, the provider URL is
`https://llm.example/v1`, and the temp dir is `/tmp/omp`. The driver's contract tests
(`internal/driver/local/omp_test.go`) replay them.

| file | what it shows | read by |
|---|---|---|
| `omp-18.4.2-rpc-control.jsonl` | the startup frames (`ready`, `setWidget`, `advisor_cost_changed`, `available_commands_update`), `get_state` (session id, thinking level), `set_model` (ok, unknown model refused), `set_thinking_level` for high/bogus/off/minimal/xhigh/max/medium each followed by `get_state`: how omp clamps levels | the fake omp: its answers and clamps |
| `omp-18.4.2-rpc-steer.jsonl` | a `steer` while `bash sleep 8` runs: the tool is detached to a background job, `agent_end` isTerminal:false, `prompt_result` settled:false, a second run started by the job's result, `session_settled` last | the log-record test |
| `omp-18.4.2-rpc-set-model-resets-thinking.jsonl` | from a live run of the driver (daemon log, not a driver script): a worker running `off`, then `set_model` to the same model: `thinking_level_changed` max before the response (the level is reset; omp says `thinking_level_changed` before the response of the command that caused it) | the fake omp: the level set_model leaves |
| `omp-18.4.2-probes.json` | small probes read back from the raw captures: launch flags (`--model`/`--thinking` unknown or clamped at start), resume variants (full id, prefix, path, other cwd, unknown id), `get_state` sent before `ready`, stdin closed mid-tool, sessions in a per-run agent dir vs `--session-dir` | the fake omp: the level omp runs at launch |
| `omp-18.4.2-launch-errors.json` | `--session-id`, an unknown `--model`, an unknown `--resume` id: stderr, no `ready` frame | the start-failure test |
