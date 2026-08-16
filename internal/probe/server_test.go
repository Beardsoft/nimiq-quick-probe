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
