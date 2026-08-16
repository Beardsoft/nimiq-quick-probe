# Nimiq Quick Probe Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Turn the hardcoded Gin consensus probe into a configurable stdlib sidecar that HAProxy can use to decide whether a Nimiq node stays in rotation.

**Architecture:** `cmd/main.go` loads env config and serves `/health` (200/503) and `/status` (always 200 JSON). `internal/probe` evaluates consensus + sync from `getSyncStatus` and peers from `getPeerCount`. Docker is multi-stage on Go 1.26; CI tests on `master`/`main` then publishes to GHCR.

**Tech Stack:** Go 1.26, stdlib `net/http` only, Docker, GitHub Actions, GHCR.

**Design:** @docs/plans/2026-08-16-nimiq-quick-probe-design.md

---

### Task 1: Bump Go to 1.26 and drop Gin

**Files:**
- Modify: `go.mod`
- Modify: `go.sum` (via `go mod tidy`)
- Modify: `cmd/main.go` (temporary stdlib stub so the module still builds)

**Step 1: Point the module at Go 1.26 and remove Gin**

```bash
go mod edit -go=1.26
go mod edit -droprequire=github.com/gin-gonic/gin
```

**Step 2: Replace `cmd/main.go` with a compiling stub**

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "probe not wired yet")
	os.Exit(1)
}
```

**Step 3: Tidy and verify the module builds**

```bash
go mod tidy
go build ./...
```

Expected: success, `go.mod` has `go 1.26` and no Gin require. `go.sum` no longer lists Gin.

**Step 4: Commit**

```bash
git add go.mod go.sum cmd/main.go
git commit -m "chore: bump Go to 1.26 and drop Gin"
```

---

### Task 2: Config loading (TDD)

**Files:**
- Create: `internal/probe/config.go`
- Create: `internal/probe/config_test.go`

**Step 1: Write the failing tests**

```go
package probe

import (
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("NIMIQ_RPC_URL", "")
	t.Setenv("LISTEN_ADDR", "")
	t.Setenv("RPC_TIMEOUT", "")
	t.Setenv("CHECK_CONSENSUS", "")
	t.Setenv("CHECK_SYNC", "")
	t.Setenv("MIN_PEERS", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RPCURL != "http://node:8648" {
		t.Fatalf("RPCURL = %q", cfg.RPCURL)
	}
	if cfg.ListenAddr != ":8080" {
		t.Fatalf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.RPCTimeout != 5*time.Second {
		t.Fatalf("RPCTimeout = %s", cfg.RPCTimeout)
	}
	if !cfg.CheckConsensus || !cfg.CheckSync {
		t.Fatalf("expected consensus and sync enabled")
	}
	if cfg.MinPeers != 1 {
		t.Fatalf("MinPeers = %d", cfg.MinPeers)
	}
}

func TestLoadConfigOverrides(t *testing.T) {
	t.Setenv("NIMIQ_RPC_URL", "http://127.0.0.1:8648")
	t.Setenv("LISTEN_ADDR", ":9090")
	t.Setenv("RPC_TIMEOUT", "2s")
	t.Setenv("CHECK_CONSENSUS", "false")
	t.Setenv("CHECK_SYNC", "0")
	t.Setenv("MIN_PEERS", "3")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RPCURL != "http://127.0.0.1:8648" {
		t.Fatalf("RPCURL = %q", cfg.RPCURL)
	}
	if cfg.ListenAddr != ":9090" {
		t.Fatalf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.RPCTimeout != 2*time.Second {
		t.Fatalf("RPCTimeout = %s", cfg.RPCTimeout)
	}
	if cfg.CheckConsensus || cfg.CheckSync {
		t.Fatalf("expected both checks disabled")
	}
	if cfg.MinPeers != 3 {
		t.Fatalf("MinPeers = %d", cfg.MinPeers)
	}
}

func TestLoadConfigInvalidTimeout(t *testing.T) {
	t.Setenv("RPC_TIMEOUT", "nope")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadConfigInvalidMinPeers(t *testing.T) {
	t.Setenv("MIN_PEERS", "-1")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadConfigInvalidBool(t *testing.T) {
	t.Setenv("CHECK_CONSENSUS", "maybe")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected error")
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/probe -run TestLoadConfig -count=1
```

Expected: FAIL, `LoadConfig` undefined.

**Step 3: Write minimal implementation**

```go
package probe

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	RPCURL         string
	ListenAddr     string
	RPCTimeout     time.Duration
	CheckConsensus bool
	CheckSync      bool
	MinPeers       int
}

func LoadConfig() (Config, error) {
	cfg := Config{
		RPCURL:         envOr("NIMIQ_RPC_URL", "http://node:8648"),
		ListenAddr:     envOr("LISTEN_ADDR", ":8080"),
		RPCTimeout:     5 * time.Second,
		CheckConsensus: true,
		CheckSync:      true,
		MinPeers:       1,
	}

	if raw := os.Getenv("RPC_TIMEOUT"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("RPC_TIMEOUT: %w", err)
		}
		cfg.RPCTimeout = d
	}

	if raw := os.Getenv("CHECK_CONSENSUS"); raw != "" {
		v, err := parseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("CHECK_CONSENSUS: %w", err)
		}
		cfg.CheckConsensus = v
	}

	if raw := os.Getenv("CHECK_SYNC"); raw != "" {
		v, err := parseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("CHECK_SYNC: %w", err)
		}
		cfg.CheckSync = v
	}

	if raw := os.Getenv("MIN_PEERS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("MIN_PEERS: %w", err)
		}
		if n < 0 {
			return Config{}, fmt.Errorf("MIN_PEERS: must be >= 0")
		}
		cfg.MinPeers = n
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseBool(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid bool %q", raw)
	}
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/probe -run TestLoadConfig -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/probe/config.go internal/probe/config_test.go
git commit -m "feat: load probe config from environment"
```

---

### Task 3: Health evaluation (TDD)

**Files:**
- Create: `internal/probe/health.go`
- Create: `internal/probe/health_test.go`

**Step 1: Write the failing tests**

```go
package probe

import "testing"

func TestEvaluateHealthy(t *testing.T) {
	cfg := Config{CheckConsensus: true, CheckSync: true, MinPeers: 1}
	sync := &SyncStatus{IsEstablished: true, RemainingBlocks: 0, StateSyncProgress: 100}
	report := Evaluate(cfg, sync, nil, 4, nil)
	if !report.Healthy {
		t.Fatalf("expected healthy, got %+v", report)
	}
	if len(report.Checks) != 3 {
		t.Fatalf("checks = %d", len(report.Checks))
	}
}

func TestEvaluateConsensusFail(t *testing.T) {
	cfg := Config{CheckConsensus: true}
	sync := &SyncStatus{IsEstablished: false, RemainingBlocks: 0, StateSyncProgress: 100}
	report := Evaluate(cfg, sync, nil, 0, nil)
	if report.Healthy {
		t.Fatal("expected unhealthy")
	}
}

func TestEvaluateSyncFail(t *testing.T) {
	cfg := Config{CheckSync: true}
	sync := &SyncStatus{IsEstablished: true, RemainingBlocks: 12, StateSyncProgress: 40}
	report := Evaluate(cfg, sync, nil, 0, nil)
	if report.Healthy {
		t.Fatal("expected unhealthy")
	}
}

func TestEvaluatePeersFail(t *testing.T) {
	cfg := Config{MinPeers: 3}
	report := Evaluate(cfg, nil, nil, 1, nil)
	if report.Healthy {
		t.Fatal("expected unhealthy")
	}
}

func TestEvaluateDisabledChecksSkipped(t *testing.T) {
	cfg := Config{CheckConsensus: false, CheckSync: false, MinPeers: 0}
	report := Evaluate(cfg, nil, nil, 0, nil)
	if !report.Healthy {
		t.Fatal("expected healthy when no checks enabled")
	}
	if len(report.Checks) != 0 {
		t.Fatalf("expected no checks, got %+v", report.Checks)
	}
}

func TestEvaluateRPCErrorFailsEnabledChecks(t *testing.T) {
	cfg := Config{CheckConsensus: true, CheckSync: true, MinPeers: 1}
	report := Evaluate(cfg, nil, errTest, 0, errTest)
	if report.Healthy {
		t.Fatal("expected unhealthy")
	}
	if len(report.Checks) != 3 {
		t.Fatalf("checks = %d", len(report.Checks))
	}
	for _, c := range report.Checks {
		if c.Passed {
			t.Fatalf("check %s should fail", c.Name)
		}
		if c.Error == "" {
			t.Fatalf("check %s missing error", c.Name)
		}
	}
}

var errTest = errString("rpc down")

type errString string

func (e errString) Error() string { return string(e) }
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/probe -run TestEvaluate -count=1
```

Expected: FAIL, `Evaluate` / `SyncStatus` undefined.

**Step 3: Write minimal implementation**

```go
package probe

import "fmt"

type SyncStatus struct {
	IsEstablished        bool   `json:"isEstablished"`
	SyncedValidityWindow bool   `json:"syncedValidityWindow"`
	CurrentBlock         uint64 `json:"currentBlock"`
	RemainingBlocks      uint64 `json:"remainingBlocks"`
	StateSyncProgress    uint64 `json:"stateSyncProgress"`
}

type CheckResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
	Error  string `json:"error,omitempty"`
}

type Report struct {
	Healthy bool          `json:"healthy"`
	Checks  []CheckResult `json:"checks"`
}

func Evaluate(cfg Config, sync *SyncStatus, syncErr error, peers int, peersErr error) Report {
	var checks []CheckResult
	healthy := true

	if cfg.CheckConsensus {
		c := CheckResult{Name: "consensus"}
		switch {
		case syncErr != nil:
			c.Error = syncErr.Error()
		case sync == nil:
			c.Error = "missing sync status"
		case !sync.IsEstablished:
			c.Detail = "consensus not established"
		default:
			c.Passed = true
			c.Detail = "established"
		}
		if !c.Passed {
			healthy = false
		}
		checks = append(checks, c)
	}

	if cfg.CheckSync {
		c := CheckResult{Name: "sync"}
		switch {
		case syncErr != nil:
			c.Error = syncErr.Error()
		case sync == nil:
			c.Error = "missing sync status"
		case sync.RemainingBlocks != 0 || sync.StateSyncProgress != 100:
			c.Detail = fmt.Sprintf("remainingBlocks=%d stateSyncProgress=%d", sync.RemainingBlocks, sync.StateSyncProgress)
		default:
			c.Passed = true
			c.Detail = fmt.Sprintf("block=%d", sync.CurrentBlock)
		}
		if !c.Passed {
			healthy = false
		}
		checks = append(checks, c)
	}

	if cfg.MinPeers > 0 {
		c := CheckResult{Name: "peers"}
		switch {
		case peersErr != nil:
			c.Error = peersErr.Error()
		case peers < cfg.MinPeers:
			c.Detail = fmt.Sprintf("%d < %d", peers, cfg.MinPeers)
		default:
			c.Passed = true
			c.Detail = fmt.Sprintf("%d >= %d", peers, cfg.MinPeers)
		}
		if !c.Passed {
			healthy = false
		}
		checks = append(checks, c)
	}

	return Report{Healthy: healthy, Checks: checks}
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/probe -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/probe/health.go internal/probe/health_test.go
git commit -m "feat: evaluate consensus, sync, and peer checks"
```

---

### Task 4: JSON-RPC client (TDD)

**Files:**
- Create: `internal/probe/rpc.go`
- Create: `internal/probe/rpc_test.go`

**Step 1: Write the failing tests**

Use `httptest` to serve Nimiq-shaped JSON-RPC envelopes. Cover success, JSON-RPC error, and HTTP 500.

```go
package probe

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRPCClientSyncStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"getSyncStatus"`) {
			t.Errorf("unexpected body %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"data":{"isEstablished":true,"syncedValidityWindow":true,"currentBlock":10,"remainingBlocks":0,"stateSyncProgress":100},"metadata":null}}`)
	}))
	defer srv.Close()

	c := NewRPCClient(srv.URL, time.Second)
	st, err := c.SyncStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !st.IsEstablished || st.CurrentBlock != 10 {
		t.Fatalf("%+v", st)
	}
}

func TestRPCClientPeerCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"data":46,"metadata":null}}`)
	}))
	defer srv.Close()

	c := NewRPCClient(srv.URL, time.Second)
	n, err := c.PeerCount(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 46 {
		t.Fatalf("peers = %d", n)
	}
}

func TestRPCClientJSONRPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"Method not found"}}`)
	}))
	defer srv.Close()

	c := NewRPCClient(srv.URL, time.Second)
	if _, err := c.PeerCount(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestRPCClientHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewRPCClient(srv.URL, time.Second)
	if _, err := c.SyncStatus(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/probe -run TestRPCClient -count=1
```

Expected: FAIL, `NewRPCClient` undefined.

**Step 3: Write minimal implementation**

```go
package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type NodeClient interface {
	SyncStatus(ctx context.Context) (SyncStatus, error)
	PeerCount(ctx context.Context) (int, error)
}

type RPCClient struct {
	url    string
	client *http.Client
}

func NewRPCClient(url string, timeout time.Duration) *RPCClient {
	return &RPCClient{
		url:    url,
		client: &http.Client{Timeout: timeout},
	}
}

func (c *RPCClient) SyncStatus(ctx context.Context) (SyncStatus, error) {
	var envelope struct {
		Data SyncStatus `json:"data"`
	}
	if err := c.call(ctx, "getSyncStatus", &envelope); err != nil {
		return SyncStatus{}, err
	}
	return envelope.Data, nil
}

func (c *RPCClient) PeerCount(ctx context.Context) (int, error) {
	var envelope struct {
		Data int `json:"data"`
	}
	if err := c.call(ctx, "getPeerCount", &envelope); err != nil {
		return 0, err
	}
	return envelope.Data, nil
}

func (c *RPCClient) call(ctx context.Context, method string, result any) error {
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  []any{},
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("rpc http %d: %s", resp.StatusCode, truncate(body, 200))
	}

	var rpc struct {
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &rpc); err != nil {
		return fmt.Errorf("rpc decode: %w", err)
	}
	if rpc.Error != nil {
		return fmt.Errorf("rpc %d: %s", rpc.Error.Code, rpc.Error.Message)
	}
	if len(rpc.Result) == 0 {
		return fmt.Errorf("rpc missing result")
	}
	return json.Unmarshal(rpc.Result, result)
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n])
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/probe -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/probe/rpc.go internal/probe/rpc_test.go
git commit -m "feat: call Nimiq JSON-RPC for sync status and peers"
```

---

### Task 5: HTTP handlers (TDD)

**Files:**
- Create: `internal/probe/server.go`
- Create: `internal/probe/server_test.go`

**Step 1: Write the failing tests**

Use a fake `NodeClient`. Assert `/health` is 200/503 and `/status` is always 200 with JSON.

```go
package probe

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeNode struct {
	sync    SyncStatus
	syncErr error
	peers   int
	peerErr error
}

func (f fakeNode) SyncStatus(context.Context) (SyncStatus, error) {
	return f.sync, f.syncErr
}

func (f fakeNode) PeerCount(context.Context) (int, error) {
	return f.peers, f.peerErr
}

func TestHealthOK(t *testing.T) {
	h := NewHandler(Config{CheckConsensus: true, CheckSync: true, MinPeers: 1}, fakeNode{
		sync:  SyncStatus{IsEstablished: true, RemainingBlocks: 0, StateSyncProgress: 100, CurrentBlock: 9},
		peers: 2,
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.Bytes())
	}
}

func TestHealthUnhealthy(t *testing.T) {
	h := NewHandler(Config{CheckConsensus: true}, fakeNode{
		sync: SyncStatus{IsEstablished: false},
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestStatusAlwaysOK(t *testing.T) {
	h := NewHandler(Config{CheckConsensus: true, MinPeers: 1}, fakeNode{
		syncErr: errors.New("dial tcp: connection refused"),
		peerErr: errors.New("dial tcp: connection refused"),
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var report Report
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Healthy {
		t.Fatal("expected unhealthy report")
	}
	if len(report.Checks) == 0 {
		t.Fatal("expected checks")
	}
}

func TestUnknownPath(t *testing.T) {
	h := NewHandler(Config{}, fakeNode{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/probe -run 'TestHealth|TestStatus|TestUnknown' -count=1
```

Expected: FAIL, `NewHandler` undefined.

**Step 3: Write minimal implementation**

Skip the unused RPC: if neither consensus nor sync is enabled, do not call `SyncStatus`. If `MinPeers == 0`, do not call `PeerCount`.

```go
package probe

import (
	"context"
	"encoding/json"
	"net/http"
)

func NewHandler(cfg Config, client NodeClient) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		report := run(r.Context(), cfg, client)
		if report.Healthy {
			writeJSON(w, http.StatusOK, report)
			return
		}
		writeJSON(w, http.StatusServiceUnavailable, report)
	})
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, run(r.Context(), cfg, client))
	})
	return mux
}

func run(ctx context.Context, cfg Config, client NodeClient) Report {
	var (
		sync    *SyncStatus
		syncErr error
		peers   int
		peerErr error
	)
	if cfg.CheckConsensus || cfg.CheckSync {
		st, err := client.SyncStatus(ctx)
		if err != nil {
			syncErr = err
		} else {
			sync = &st
		}
	}
	if cfg.MinPeers > 0 {
		peers, peerErr = client.PeerCount(ctx)
	}
	return Evaluate(cfg, sync, syncErr, peers, peerErr)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/probe -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/probe/server.go internal/probe/server_test.go
git commit -m "feat: serve /health and /status"
```

---

### Task 6: Wire `cmd/main.go`

**Files:**
- Modify: `cmd/main.go`

**Step 1: Replace the stub**

```go
package main

import (
	"log"
	"net/http"

	"github.com/Beardsoft/nimiq-quick-probe/internal/probe"
)

func main() {
	cfg, err := probe.LoadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	client := probe.NewRPCClient(cfg.RPCURL, cfg.RPCTimeout)
	handler := probe.NewHandler(cfg, client)

	log.Printf("listening on %s, rpc %s", cfg.ListenAddr, cfg.RPCURL)
	if err := http.ListenAndServe(cfg.ListenAddr, handler); err != nil {
		log.Fatal(err)
	}
}
```

**Step 2: Build and test**

```bash
go test ./...
go build -o /tmp/health-probe ./cmd
```

Expected: tests pass, binary builds.

**Step 3: Commit**

```bash
git add cmd/main.go
git commit -m "feat: start the probe HTTP server"
```

---

### Task 7: Multi-stage Dockerfile and compose

**Files:**
- Modify: `Dockerfile`
- Modify: `docker-compose.yml`

**Step 1: Rewrite the Dockerfile**

```dockerfile
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /health-probe ./cmd

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /health-probe /health-probe
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/health-probe"]
```

**Step 2: Rewrite compose so the probe can sit next to a node**

```yaml
services:
  health-probe:
    build: .
    ports:
      - "9090:8080"
    environment:
      NIMIQ_RPC_URL: http://node:8648
      LISTEN_ADDR: ":8080"
      RPC_TIMEOUT: 5s
      CHECK_CONSENSUS: "true"
      CHECK_SYNC: "true"
      MIN_PEERS: "1"
    extra_hosts:
      - "node:host-gateway"
```

`extra_hosts` is a local convenience; in Swarm/Compose with a real `node` service, remove it and attach both services to the same network.

**Step 3: Build the image**

```bash
docker build -t nimiq-quick-probe:local .
```

Expected: image builds.

**Step 4: Commit**

```bash
git add Dockerfile docker-compose.yml
git commit -m "build: multi-stage Go 1.26 image and documented compose env"
```

---

### Task 8: Fix GitHub Actions

**Files:**
- Modify: `.github/workflows/docker-image.yml`

**Step 1: Replace the workflow**

Trigger on `master` and `main`. Run tests before build. Keep GHCR publish off PRs. Use current action versions.

```yaml
name: Docker

on:
  push:
    branches: [master, main]
    tags: ["v*.*.*"]
  pull_request:
    branches: [master, main]

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.26"
      - run: go test ./...

  build:
    needs: test
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-buildx-action@v3
      - uses: docker/login-action@v3
        if: github.event_name != 'pull_request'
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
      - uses: docker/build-push-action@v6
        with:
          context: .
          push: ${{ github.event_name != 'pull_request' }}
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

**Step 2: Commit**

```bash
git add .github/workflows/docker-image.yml
git commit -m "ci: test on Go 1.26 and build images from master"
```

---

### Task 9: README

**Files:**
- Create: `README.md`

**Step 1: Write the README**

Include: what it is, env table, `/health` vs `/status`, Docker/compose, HAProxy snippet, local build.

```markdown
# nimiq-quick-probe

Sidecar that tells HAProxy whether a Nimiq node should stay in rotation.

`GET /health` returns **200** when every enabled check passes and **503** otherwise.
`GET /status` always returns **200** with a JSON report so you can inspect a failing node.

## Checks

| Check | RPC | Pass when |
|---|---|---|
| Consensus | `getSyncStatus` | `isEstablished` |
| Sync | `getSyncStatus` | `remainingBlocks == 0` and `stateSyncProgress == 100` |
| Peers | `getPeerCount` | `>= MIN_PEERS` |

## Configuration

| Variable | Default | Role |
|---|---|---|
| `NIMIQ_RPC_URL` | `http://node:8648` | Node JSON-RPC |
| `LISTEN_ADDR` | `:8080` | Probe bind address |
| `RPC_TIMEOUT` | `5s` | Per-request timeout |
| `CHECK_CONSENSUS` | `true` | Require consensus |
| `CHECK_SYNC` | `true` | Require fully synced |
| `MIN_PEERS` | `1` | Minimum peers; `0` disables the peer check |

## Docker

```bash
docker compose up --build
curl -i http://127.0.0.1:9090/health
curl -s http://127.0.0.1:9090/status
```

Image: `ghcr.io/beardsoft/nimiq-quick-probe`.

## HAProxy

```
backend nimiq_rpc
    option httpchk GET /health
    http-check expect status 200
    server node1 10.0.0.10:8648 check inter 2s fall 3 rise 2
    server probe1 10.0.0.10:8080
```

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
```

**Step 2: Commit**

```bash
git add README.md
git commit -m "docs: explain HAProxy usage and probe configuration"
```

---

### Task 10: Final verification

**Files:** none (run only)

**Step 1: Run the full test suite**

```bash
go test ./...
```

Expected: PASS.

**Step 2: Confirm the image still builds if Docker is available**

```bash
docker build -t nimiq-quick-probe:local .
```

Expected: success, or skip if Docker is unavailable and note it.

**Step 3: No extra commit unless something failed and needed a fix.**
