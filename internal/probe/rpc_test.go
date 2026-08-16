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

func TestRPCClientConsensusEstablished(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"isConsensusEstablished"`) {
			t.Errorf("unexpected body %s", body)
		}
		io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"data":true,"metadata":null}}`)
	}))
	defer srv.Close()

	c := NewRPCClient(srv.URL, time.Second)
	ok, err := c.ConsensusEstablished(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected consensus")
	}
}

func TestRPCClientLatestBlock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"getLatestBlock"`) {
			t.Errorf("unexpected body %s", body)
		}
		if !strings.Contains(string(body), `"params":[false]`) {
			t.Errorf("expected includeBody=false, got %s", body)
		}
		io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"data":{"number":8908524,"timestamp":1786912995245},"metadata":null}}`)
	}))
	defer srv.Close()

	c := NewRPCClient(srv.URL, time.Second)
	head, err := c.LatestBlock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if head.Number != 8908524 || head.Timestamp != 1786912995245 {
		t.Fatalf("%+v", head)
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
	if _, err := c.LatestBlock(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}
