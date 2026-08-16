package probe

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeNode struct {
	consensus    bool
	consensusErr error
	head         Head
	headErr      error
	peers        int
	peerErr      error
}

func (f fakeNode) ConsensusEstablished(context.Context) (bool, error) {
	return f.consensus, f.consensusErr
}

func (f fakeNode) LatestBlock(context.Context) (Head, error) {
	return f.head, f.headErr
}

func (f fakeNode) PeerCount(context.Context) (int, error) {
	return f.peers, f.peerErr
}

func TestHealthOK(t *testing.T) {
	h := NewHandler(Config{CheckConsensus: true, CheckSync: true, MaxBlockAge: 15 * time.Second, MinPeers: 1}, fakeNode{
		consensus: true,
		head:      Head{Number: 9, Timestamp: time.Now().UnixMilli()},
		peers:     2,
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.Bytes())
	}
}

func TestHealthUnhealthy(t *testing.T) {
	h := NewHandler(Config{CheckConsensus: true}, fakeNode{
		consensus: false,
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestHealthStaleHead(t *testing.T) {
	h := NewHandler(Config{CheckSync: true, MaxBlockAge: 15 * time.Second}, fakeNode{
		head: Head{Number: 9, Timestamp: time.Now().Add(-30 * time.Second).UnixMilli()},
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.Bytes())
	}
}

func TestStatusAlwaysOK(t *testing.T) {
	h := NewHandler(Config{CheckConsensus: true, MinPeers: 1}, fakeNode{
		consensusErr: errors.New("dial tcp: connection refused"),
		peerErr:      errors.New("dial tcp: connection refused"),
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
