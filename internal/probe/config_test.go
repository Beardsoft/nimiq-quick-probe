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
