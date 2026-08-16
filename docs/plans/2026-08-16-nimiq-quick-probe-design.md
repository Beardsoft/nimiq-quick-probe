# Nimiq Quick Probe Design

Date: 2026-08-16

## Problem

`nimiq-quick-probe` sits next to a Nimiq node and answers HAProxy health checks. Today it is a single Gin endpoint that calls `isConsensusEstablished` on a hardcoded `http://node:8648`. Go is 1.20 in `go.mod` (Dockerfile is 1.22). There is no README, no configuration, and the GitHub Actions workflow only runs on `main` while the default branch is `master`.

## Goal

Ship a small, configurable sidecar that HAProxy can poll to decide whether a Nimiq node should stay in rotation, plus a status URL operators can inspect when a node is taken out.

## Approach

Stdlib `net/http` sidecar. Drop Gin. Environment-configured checks. Defaults: consensus + fully synced + at least one peer.

## Architecture

```
HAProxy  -- GET /health -->  probe :8080  -- JSON-RPC -->  node :8648
ops      -- GET /status -->
```

- `/health` is for HAProxy. **200** if every enabled check passes, **503** otherwise.
- `/status` is for operators. Always **200**, with per-check JSON so a failing node can be inspected without HAProxy treating this URL as a health check.

On each request the probe calls:

- `getSyncStatus` when consensus or sync is enabled
- `getPeerCount` when `MIN_PEERS > 0`

Live Nimiq RPC envelope is `{ "data": ..., "metadata": ... }`.

| Check | Source | Pass when |
|---|---|---|
| Consensus | `getSyncStatus.data.isEstablished` | `true` |
| Sync | `getSyncStatus.data` | `remainingBlocks == 0` and `stateSyncProgress == 100` |
| Peers | `getPeerCount.data` | `>= MIN_PEERS` |

Skip the RPC call for a disabled check.

## Configuration

| Variable | Default | Role |
|---|---|---|
| `NIMIQ_RPC_URL` | `http://node:8648` | Node JSON-RPC |
| `LISTEN_ADDR` | `:8080` | Probe bind address |
| `RPC_TIMEOUT` | `5s` | Per-request timeout |
| `CHECK_CONSENSUS` | `true` | Require consensus |
| `CHECK_SYNC` | `true` | Require fully synced |
| `MIN_PEERS` | `1` | Minimum peers; `0` disables the peer check |

Invalid env at startup: log a clear error and exit. Do not listen.

## Components

- `cmd/main.go` — load config, start HTTP server
- `internal/probe` — env config, JSON-RPC client, check evaluation, `/health` and `/status` handlers
- `README.md` — env table, compose sidecar, HAProxy `httpchk` snippet
- `Dockerfile` — multi-stage, Go 1.26
- `docker-compose.yml` — documented env next to a `node` service
- `.github/workflows/docker-image.yml` — test, then build/push

## Error handling

- RPC timeout, transport error, or JSON-RPC error: that check fails; `/health` is 503
- `/status` includes the error string on the failed check
- No check cache. HAProxy polling a few times per second is cheap enough for two RPC calls

## Testing

Table tests for evaluation (each check on/off, RPC failure). `httptest` for `/health` and `/status` status codes. CI does not need a live node.

## Docker and CI

Dockerfile: `golang:1.26-alpine` builder, `CGO_ENABLED=0`, small runtime image (`distroless` or `alpine`). Binary `/health-probe`.

Workflow:

- Trigger on `master` and `main`, `v*.*.*` tags, and PRs against those branches
- `test` job: Go 1.26, `go test ./...`, must pass before build
- Keep GHCR publish on non-PR events
- Bump checkout / buildx / login / metadata / build-push to current versions

## Out of scope

Prometheus metrics, check-result caching, circuit breakers, `/live` vs `/ready`, Kubernetes probes beyond what `/health` already covers.
