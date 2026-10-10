# Development and verification

Use **Go 1.27.2** (go.mod is also the CI toolchain source). Keep third-party versions unchanged unless an audit or measurement justifies an update.

## Dependencies and standard startup

- Windows: Go + Git. Normal release builds use CGO_ENABLED=0 and need no C compiler. A Windows race build additionally needs a compatible GCC; neither the current Windows environment nor WSL has one.
- Linux: Go, GCC, pkg-config, libgtk-3-dev, libayatana-appindicator3-dev; a desktop session is needed for native tray verification.
- macOS: Go and Xcode command-line tools; follow the deployment target required by the tray library.
- No Node.js, Python or browser runtime is needed by this application.

Build with `GOFIPS140=v1.0.0 go build -o isllmalive.exe ./cmd/isllmalive`, then launch the binary. It reads config.json next to itself. Use a dedicated temporary directory for tests; never delete or overwrite the real user configuration to exercise defaults.

## Checks

```sh
export GOFIPS140=v1.0.0 GODEBUG=fips140=off
gofmt -l .
go test -count=1 -mod=readonly ./...
go vet -mod=readonly ./...
go mod tidy -diff
go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 -show verbose ./...
# Linux with development packages:
go test -race -count=1 -mod=readonly ./...
```

Windows release parameters: `GOFIPS140=v1.0.0 CGO_ENABLED=0 go build -trimpath -mod=readonly -ldflags "-s -w -H=windowsgui" -o isllmalive.exe ./cmd/isllmalive` (set the environment variable using the syntax of your shell).
This session uses `-buildvcs=false` only for local sandbox checks because sandbox Git ownership differs; CI and standard release builds retain VCS metadata.

PR/push checks cover format, tests, vet, module hygiene, vulnerability scanning and Linux race. Existing cross-platform release builds remain separate. Merely adding the workflow does not mean remote CI ran.

The supported Go Cryptographic Module v1.0.0 avoids the 32 MiB static DRBG scratch buffer in the current standard library. Use Go 1.27.2 for the standard-library security fixes. go.mod keeps FIPS mode off by default; HTTPS certificate checks and secure randomness remain enabled. GOFIPS140=off builds remain available as a comparison. Frozen modules may lack newer cryptography APIs, so repeat compatibility checks when changing providers or toolchains.

## Diagnostics and performance

Opening diagnostics includes live-heap metrics, runtime memory, goroutine count, last round duration and HTTP request count. `--profile-dir <directory>` explicitly enables local heap/allocs/goroutine pprof files when diagnostics is opened. No debug HTTP port is exposed. Profiles can contain sensitive process data; keep them private. No forced GC or working-set trimming is used.

See plans/performance-baseline.md for reproducible parser/icon benchmarks. These are temporary allocations, not process working set. The original <10MB target is still unmet. Windows 5-monitor and 20-monitor release builds each require 24h sampling of private working set, private bytes, live heap, peaks, CPU and refresh latency before acceptance; GUI and shutdown behavior must be checked separately.

## Dependency audit scope

The module tidy baseline found no unused direct or indirect dependency. uTLS and its compression/crypto dependencies are still used. The initial reachable-symbol govulncheck scan found no reachable vulnerability, but reported vulnerabilities in uncalled imported code/modules. This is not a full security guarantee; see [the recorded advisory list](plans/dependency-audit.md) and review dependency changes separately.

## References

- [Effective Go](https://go.dev/doc/effective_go)
- [Go 1.24: testing.Context and B.Loop](https://go.dev/doc/go1.24)
- [Go 1.25: WaitGroup.Go](https://go.dev/doc/go1.25)
- [Modern Go guidelines](https://github.com/JetBrains/go-modern-guidelines)
- [Go vulnerability checking](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck)

The root application context intentionally uses context.Background. Tests use t.Context; WaitGroup.Go is used for worker batches. Explicit severity logic is retained because partial Unknown is not equivalent to numeric max.

Long-run sampling: see [Windows validation](plans/windows-validation.md). Explicit options `--metrics-file`, `--metrics-interval` and `--run-for` enable private JSONL sampling and automatic exit; defaults leave sampling off and normal lifetime unchanged.
