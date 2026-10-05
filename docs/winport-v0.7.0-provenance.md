# Winport v0.7.0 — Rebase provenance

Base: upstream tag v0.7.0 of sting8k/piggery.
Result: fork main (winport) = v0.7.0 sources + winport patch series.

## What changed in v0.7.0 that affects the winport

- `internal/server/hook.go`: upstream refactored notify hooks (51 → 79 lines).
  The winport patch's context lines no longer matched at the expected offset.
  **Manual fix**: replaced the two Unix-only lines (Setpgid + Kill) with
  `setupHookKill(cmd)` — same pattern as the original patch. All other
  patch hunks applied cleanly via `git apply --3way`.

- `internal/server/server.go`: applied cleanly (no conflict).

- All other winported files: applied cleanly.

## Gates

- `GOOS=linux go build ./cmd/piggery`: PASS
- `GOOS=windows GOARmd64 go build` (with version stamp): PASS

## Commit

- Fork main: c512756 (full v0.7.0 tree + winport)
- Tag: v0.7.0+winport
- Binary sha256: 7baa1c00060819915df331af03d5e6d9d934ea9763fb75340e58cb9128964602
