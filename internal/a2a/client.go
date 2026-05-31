package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is an A2A protocol client for sending tasks to other A2A servers.
type Client struct {
	httpClient *http.Client
	baseURL    string
	authToken  string
}

// NewClient creates a new A2A client.
func NewClient(baseURL, authToken string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 300 * time.Second},
		baseURL:    baseURL,
		authToken:  authToken,
	}
}

// SendMessage sends a message to an A2A server (sync response).
func (c *Client) SendMessage(ctx context.Context, taskID string, msg *Message) (*Task, error) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "message/send",
		Params: mustMarshal(SendMessageParams{
			TaskID:  taskID,
			Message: msg,
		}),
		ID: 1,
	}

	var result Task
	if err := c.doRPC(ctx, &req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SendMessageStream sends a message and returns SSE events via channel.
func (c *Client) SendMessageStream(ctx context.Context, taskID string, msg *Message) (<-chan TaskEvent, error) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "message/send",
		Params: mustMarshal(SendMessageParams{
			TaskID:  taskID,
			Message: msg,
		}),
		ID: 1,
	}

	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/a2a", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if c.authToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("a2a request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("a2a request: status %d", resp.StatusCode)
	}

	ch := make(chan TaskEvent, 100)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		c.readSSE(ctx, resp.Body, ch)
	}()

	return ch, nil
}

// GetTask gets the current state of a task.
func (c *Client) GetTask(ctx context.Context, taskID string) (*Task, error) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "task/get",
		Params:  mustMarshal(map[string]string{"task_id": taskID}),
		ID:      2,
	}

	var result Task
	if err := c.doRPC(ctx, &req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CancelTask cancels a running task.
func (c *Client) CancelTask(ctx context.Context, taskID string) (*Task, error) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "task/cancel",
		Params:  mustMarshal(map[string]string{"task_id": taskID}),
		ID:      3,
	}

	var result Task
	if err := c.doRPC(ctx, &req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetAgentCard retrieves the Agent Card from the server.
func (c *Client) GetAgentCard(ctx context.Context) (*AgentCard, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/.well-known/agent.json", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("get agent card: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get agent card: status %d", resp.StatusCode)
	}

	var card AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, fmt.Errorf("decode agent card: %w", err)
	}
	return &card, nil
}

// doRPC performs a JSON-RPC call and decodes the result.
func (c *Client) doRPC(ctx context.Context, req *JSONRPCRequest, result any) error {
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/a2a", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("a2a rpc: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return fmt.Errorf("a2a error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	if result != nil && rpcResp.Result != nil {
		data, _ := json.Marshal(rpcResp.Result)
		return json.Unmarshal(data, result)
	}
	return nil
}

// readSSE reads SSE events from the response body.
func (c *Client) readSSE(ctx context.Context, body io.Reader, ch chan<- TaskEvent) {
	buf := make([]byte, 4096)
	var remaining []byte

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := body.Read(buf)
		if n > 0 {
			remaining = append(remaining, buf[:n]...)
			// Parse SSE lines
			for {
				idx := bytes.Index(remaining, []byte("\n\n"))
				if idx < 0 {
					break
				}
				line := remaining[:idx]
				remaining = remaining[idx+2:]

				// Parse "data: ..."
				if bytes.HasPrefix(line, []byte("data: ")) {
					data := line[6:]
					var event TaskEvent
					if err := json.Unmarshal(data, &event); err == nil {
						select {
						case ch <- event:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}
		if err != nil {
			return
		}
	}
}

func mustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
