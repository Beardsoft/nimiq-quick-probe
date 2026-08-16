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
	MaxBlockAge    time.Duration
	MinPeers       int
}

func LoadConfig() (Config, error) {
	cfg := Config{
		RPCURL:         envOr("NIMIQ_RPC_URL", "http://node:8648"),
		ListenAddr:     envOr("LISTEN_ADDR", ":8080"),
		RPCTimeout:     5 * time.Second,
		CheckConsensus: true,
		CheckSync:      true,
		MaxBlockAge:    15 * time.Second,
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

	if raw := os.Getenv("MAX_BLOCK_AGE"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("MAX_BLOCK_AGE: %w", err)
		}
		if d < 0 {
			return Config{}, fmt.Errorf("MAX_BLOCK_AGE: must be >= 0")
		}
		cfg.MaxBlockAge = d
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
