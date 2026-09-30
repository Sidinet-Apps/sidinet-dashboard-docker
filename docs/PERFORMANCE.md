# Performance validation

v0.11.0 adds provider TTL caching, singleflight, a bounded provider collector pool, HTTP limits and internal diagnostics.

## Targets for Raspberry Pi validation

These are acceptance targets, not measurements from CI:

- Idle CPU: <1% after warm-up.
- Normal dashboard CPU: Pi 4 <5%, Pi 5 <3%.
- Idle RAM target: <=100 MB; normal <=150 MB; hard target <=200 MB.
- 72-hour run: no unbounded growth in goroutines, heap, DB, logs or timers.
- Stress dataset: 100 services, 50 shortcuts, 30 widgets, 10 pages, 20 visible widgets and 30 monitors.

## Local/CI checks

Run `go test ./...` and query `/api/v1/diagnostics/performance` during load. Compare `go_heap_bytes`, `go_sys_bytes`, `goroutines`, request counters and provider cache statistics before/after the test.

Hardware numbers must be recorded only from the ARM64 image running on the actual Pi 4/Pi 5. Do not treat workstation or CI results as Raspberry Pi measurements.
