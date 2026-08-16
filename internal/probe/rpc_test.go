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
