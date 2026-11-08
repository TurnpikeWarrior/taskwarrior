# Verification

The local candidate is `0.4.0-milestone4`. Core functionality, groups and colors,
TOML configuration, and the style editor are implemented against SPEC.md.

## Checks performed

- `go test ./...`, `go test -race ./...`, `go vet ./...`, and native builds passed
  on macOS ARM64 and Linux ARM64 in a container VM.
- All three milestone smoke flows passed on both environments, including failed
  writes, draft retention, undo, settings, and restart persistence.
- macOS/Linux/Windows candidates compiled for amd64 and arm64. Windows execution
  was not tested and is outside the current verification scope.
- Six terminal protocol profiles passed on macOS and Linux: truecolor with dark
  and light settings, ANSI 256, ANSI 16, monochrome/ASCII, and disabled mouse.
- A private tmux session passed keyboard, resize, persistence, and exit checks.
- Native GUI visual inspection was unavailable. Physical fonts, perceived contrast,
  and hardware pointer behavior are not certified by the PTY tests.

## Performance summary

Measurements used a local Apple Silicon machine, Go 1.27.3, an 80×24 Unix PTY,
and cached filesystem data. Each startup sample used a fresh process; disk caches
were not purged. The large fixture contained 5,000 active and 5,000 completed
ASCII-title tasks, with no groups. Each startup scenario had 25 samples.

| Measurement | Result |
| --- | --- |
| Empty startup median / p95 | 29.70 / 30.94 ms |
| Empty first launch | 142.01 ms |
| 10,000-task startup median / p95 | 38.82 / 46.68 ms |
| 10,000-task End / Home median | 16.57 / 16.63 ms |
| Idle resident memory, empty / 10,000 tasks | 8.61 / 30.06 MiB |
| Idle CPU, empty / 10,000 tasks | About 0.60% / 0.40% of one CPU over five seconds |

Visible rows are reused until a task revision, visibility, or snapshot change.
This reduced model navigation allocations from about 38.8 MB to 8.2 KB per event.
The under-50-ms startup goal is met by the median and p95, but not the first-launch
outlier. Its application/OS/loader contributions have not been isolated.

These are historical reference measurements, not portable performance guarantees.
Generated logs, profiles, binary fingerprints, local paths, and machine-specific
reports are excluded from version control. Reproduce current results locally:

```sh
go build -o bin/taskwarrior ./cmd/taskwarrior
go test ./...
go test -race ./...
go vet ./...
python3 scripts/terminal_smoke.py
python3 scripts/groups_smoke.py
python3 scripts/style_smoke.py
python3 scripts/release_compatibility.py
python3 scripts/release_performance.py --output artifacts/release/performance.json
go test ./internal/app -run '^$' -bench 'Benchmark(Navigation|Startup)' -benchmem -benchtime=1s -count=3
python3 scripts/release_builds.py
```

Generated evidence goes under ignored `artifacts/release/`; candidate executables
and checksums go under ignored `bin/release/`.
