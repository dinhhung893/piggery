-- v19 -> v20: a session's own transcript, as its adapter reported it at join.auto (the harness's
-- file: pi's session file, Claude's or Codex's transcript_path) and that file's format (pi, claude,
-- codex). The CLI reads it for the session's ctx, turns and tail; the daemon never does. NULL: not
-- reported (a headless worker, an older adapter).
ALTER TABLE participants ADD COLUMN transcript TEXT;
ALTER TABLE participants ADD COLUMN transcript_format TEXT;
