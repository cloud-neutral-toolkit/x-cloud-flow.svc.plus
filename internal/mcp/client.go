package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ClientOptions struct {
	BearerToken string
}

type Client struct {
	BaseURL     string
	HTTP        *http.Client
	BearerToken string
}

func NewClient(baseURL string) *Client {
	return NewClientWithOptions(baseURL, ClientOptions{})
}

func NewClientWithOptions(baseURL string, opts ClientOptions) *Client {
	return &Client{
		BaseURL:     baseURL,
		HTTP:        &http.Client{Timeout: 15 * time.Second},
		BearerToken: opts.BearerToken,
	}
}

func (c *Client) ToolsList(ctx context.Context) ([]Tool, error) {
	req := rpcReq{JSONRPC: "2.0", ID: 1, Method: "tools/list"}
	var resp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      any    `json:"id"`
		Result  struct {
			Tools []Tool `json:"tools"`
		} `json:"result"`
		Error *rpcErr `json:"error"`
	}
	if err := c.call(ctx, req, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf(resp.Error.Message)
	}
	return resp.Result.Tools, nil
}

func (c *Client) ToolsCall(ctx context.Context, name string, arguments any) (any, error) {
	req := rpcReq{JSONRPC: "2.0", ID: 1, Method: "tools/call"}
	payload, err := json.Marshal(map[string]any{
		"name":      name,
		"arguments": arguments,
	})
	if err != nil {
		return nil, err
	}
	req.Params = payload

	var resp struct {
		JSONRPC string  `json:"jsonrpc"`
		ID      any     `json:"id"`
		Result  any     `json:"result"`
		Error   *rpcErr `json:"error"`
	}
	if err := c.call(ctx, req, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf(resp.Error.Message)
	}
	return resp.Result, nil
}

func (c *Client) call(ctx context.Context, req any, out any) error {
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.BearerToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.BearerToken)
	}
	res, err := c.HTTP.Do(httpReq)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		return fmt.Errorf("http %d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(out)
}
