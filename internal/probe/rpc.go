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
	ConsensusEstablished(ctx context.Context) (bool, error)
	LatestBlock(ctx context.Context) (Head, error)
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

func (c *RPCClient) ConsensusEstablished(ctx context.Context) (bool, error) {
	var envelope struct {
		Data bool `json:"data"`
	}
	if err := c.call(ctx, "isConsensusEstablished", nil, &envelope); err != nil {
		return false, err
	}
	return envelope.Data, nil
}

func (c *RPCClient) LatestBlock(ctx context.Context) (Head, error) {
	var envelope struct {
		Data Head `json:"data"`
	}
	if err := c.call(ctx, "getLatestBlock", []any{false}, &envelope); err != nil {
		return Head{}, err
	}
	return envelope.Data, nil
}

func (c *RPCClient) PeerCount(ctx context.Context) (int, error) {
	var envelope struct {
		Data int `json:"data"`
	}
	if err := c.call(ctx, "getPeerCount", nil, &envelope); err != nil {
		return 0, err
	}
	return envelope.Data, nil
}

func (c *RPCClient) call(ctx context.Context, method string, params []any, result any) error {
	if params == nil {
		params = []any{}
	}
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
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
		Error *struct {
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
