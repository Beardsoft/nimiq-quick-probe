package probe

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
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
		consensus    *bool
		consensusErr error
		head         *Head
		headErr      error
		peers        int
		peerErr      error
	)
	if cfg.CheckConsensus {
		ok, err := client.ConsensusEstablished(ctx)
		if err != nil {
			consensusErr = err
		} else {
			consensus = &ok
		}
	}
	if cfg.CheckSync && cfg.MaxBlockAge > 0 {
		h, err := client.LatestBlock(ctx)
		if err != nil {
			headErr = err
		} else {
			head = &h
		}
	}
	if cfg.MinPeers > 0 {
		peers, peerErr = client.PeerCount(ctx)
	}
	return Evaluate(cfg, time.Now(), consensus, consensusErr, head, headErr, peers, peerErr)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
