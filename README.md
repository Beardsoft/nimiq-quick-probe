# nimiq-quick-probe

Sidecar that tells HAProxy whether a Nimiq node should stay in rotation.

`GET /health` returns **200** when every enabled check passes and **503** otherwise.
`GET /status` always returns **200** with a JSON report so you can inspect a failing node.

## Checks

| Check | RPC | Pass when |
|---|---|---|
| Consensus | `isConsensusEstablished` | `true` |
| Head age | `getLatestBlock` | head timestamp newer than `MAX_BLOCK_AGE` |
| Peers | `getPeerCount` | `>= MIN_PEERS` |

## Configuration

| Variable | Default | Role |
|---|---|---|
| `NIMIQ_RPC_URL` | `http://node:8648` | Node JSON-RPC |
| `LISTEN_ADDR` | `:8080` | Probe bind address |
| `RPC_TIMEOUT` | `5s` | Per-request timeout |
| `CHECK_CONSENSUS` | `true` | Require consensus |
| `CHECK_SYNC` | `true` | Require a fresh head |
| `MAX_BLOCK_AGE` | `15s` | Fail if the latest block is older than this; `0` disables |
| `MIN_PEERS` | `1` | Minimum peers; `0` disables the peer check |

## Docker

```bash
docker compose up --build
curl -i http://127.0.0.1:9090/health
curl -s http://127.0.0.1:9090/status
```

Image: `ghcr.io/beardsoft/nimiq-quick-probe`.

## HAProxy

Point `httpchk` at the **probe**, not the node. A typical layout is node `:8648` plus probe `:8080` on the same host; HAProxy checks the probe and proxies RPC to the node.

```
backend nimiq_rpc
    option httpchk
    http-check send meth GET uri /health
    http-check expect status 200
    server node1 10.0.0.10:8648 check port 8080 inter 2s fall 3 rise 2
```

`check port 8080` keeps traffic on the node RPC port and health checks on the probe.

## Build

```bash
go test ./...
go build -o health-probe ./cmd
```
