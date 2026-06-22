package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{httpClient: &http.Client{}}
}

func (c *Client) Discover(ctx context.Context, server Server, secret ServerSecret) ([]DiscoveredTool, error) {
	switch server.TransportType {
	case "http":
		return c.discoverHTTP(ctx, server, secret)
	case "stdio":
		return c.discoverStdio(ctx, server, secret)
	default:
		return nil, errors.New("unsupported MCP transport")
	}
}

func (c *Client) CallTool(ctx context.Context, server Server, secret ServerSecret, name string, arguments string) (ToolCallResult, error) {
	switch server.TransportType {
	case "http":
		return c.callHTTP(ctx, server, secret, name, arguments)
	case "stdio":
		return c.callStdio(ctx, server, secret, name, arguments)
	default:
		return ToolCallResult{}, errors.New("unsupported MCP transport")
	}
}

func (c *Client) discoverHTTP(ctx context.Context, server Server, secret ServerSecret) ([]DiscoveredTool, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout(server.RequestTimeoutMS))
	defer cancel()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": "tools-list", "method": "tools/list", "params": map[string]any{}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.HTTPURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")
	for key, value := range secret.HTTPHeaders {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, errors.New(strings.TrimSpace(string(payload)))
	}
	return parseToolsResponse(resp.Body)
}

func (c *Client) callHTTP(ctx context.Context, server Server, secret ServerSecret, name string, arguments string) (ToolCallResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout(server.RequestTimeoutMS))
	defer cancel()
	body, err := json.Marshal(toolCallRequest(name, arguments))
	if err != nil {
		return ToolCallResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.HTTPURL, bytes.NewReader(body))
	if err != nil {
		return ToolCallResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")
	for key, value := range secret.HTTPHeaders {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ToolCallResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return ToolCallResult{}, errors.New(strings.TrimSpace(string(payload)))
	}
	return parseToolCallResponse(resp.Body)
}

func (c *Client) discoverStdio(ctx context.Context, server Server, secret ServerSecret) ([]DiscoveredTool, error) {
	if strings.TrimSpace(server.Command) == "" {
		return nil, errors.New("stdio command is required")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout(server.StartupTimeoutMS+server.RequestTimeoutMS))
	defer cancel()
	cmd := exec.CommandContext(ctx, server.Command, server.Arguments...)
	if server.WorkingDirectory != "" {
		cmd.Dir = server.WorkingDirectory
	}
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
	for key, value := range secret.Environment {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := &boundedBuffer{limit: 64 * 1024}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	request := `{"jsonrpc":"2.0","id":"tools-list","method":"tools/list","params":{}}` + "\n"
	if _, err := stdin.Write([]byte(request)); err != nil {
		_ = cmd.Process.Kill()
		return nil, err
	}
	_ = stdin.Close()
	reader := bufio.NewReader(io.LimitReader(stdout, 2*1024*1024))
	line, err := reader.ReadBytes('\n')
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, err
	}
	_ = cmd.Wait()
	return parseToolsResponse(bytes.NewReader(line))
}

func (c *Client) callStdio(ctx context.Context, server Server, secret ServerSecret, name string, arguments string) (ToolCallResult, error) {
	if strings.TrimSpace(server.Command) == "" {
		return ToolCallResult{}, errors.New("stdio command is required")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout(server.StartupTimeoutMS+server.RequestTimeoutMS))
	defer cancel()
	cmd := exec.CommandContext(ctx, server.Command, server.Arguments...)
	if server.WorkingDirectory != "" {
		cmd.Dir = server.WorkingDirectory
	}
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
	for key, value := range secret.Environment {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return ToolCallResult{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ToolCallResult{}, err
	}
	stderr := &boundedBuffer{limit: 64 * 1024}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return ToolCallResult{}, err
	}
	request, err := json.Marshal(toolCallRequest(name, arguments))
	if err != nil {
		_ = cmd.Process.Kill()
		return ToolCallResult{}, err
	}
	if _, err := stdin.Write(append(request, '\n')); err != nil {
		_ = cmd.Process.Kill()
		return ToolCallResult{}, err
	}
	_ = stdin.Close()
	reader := bufio.NewReader(io.LimitReader(stdout, 2*1024*1024))
	line, err := reader.ReadBytes('\n')
	if err != nil {
		_ = cmd.Process.Kill()
		return ToolCallResult{}, err
	}
	_ = cmd.Wait()
	return parseToolCallResponse(bytes.NewReader(line))
}

func parseToolsResponse(reader io.Reader) ([]DiscoveredTool, error) {
	var payload struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				InputSchema any    `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(reader, 2*1024*1024)).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Error != nil {
		return nil, errors.New(payload.Error.Message)
	}
	tools := make([]DiscoveredTool, 0, len(payload.Result.Tools))
	for _, tool := range payload.Result.Tools {
		if strings.TrimSpace(tool.Name) == "" {
			continue
		}
		tools = append(tools, DiscoveredTool{Name: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema})
	}
	return tools, nil
}

func toolCallRequest(name string, arguments string) map[string]any {
	var args any = map[string]any{}
	if strings.TrimSpace(arguments) != "" {
		var parsed any
		if err := json.Unmarshal([]byte(arguments), &parsed); err == nil {
			args = parsed
		}
	}
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      "tools-call",
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": args,
		},
	}
}

func parseToolCallResponse(reader io.Reader) (ToolCallResult, error) {
	var payload struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			StructuredContent any  `json:"structuredContent"`
			IsError           bool `json:"isError"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(reader, 2*1024*1024)).Decode(&payload); err != nil {
		return ToolCallResult{}, err
	}
	if payload.Error != nil {
		return ToolCallResult{}, errors.New(payload.Error.Message)
	}
	if payload.Result.IsError {
		return ToolCallResult{}, errors.New("MCP tool returned an error result")
	}
	var builder strings.Builder
	for _, item := range payload.Result.Content {
		if item.Type == "text" && strings.TrimSpace(item.Text) != "" {
			if builder.Len() > 0 {
				builder.WriteString("\n")
			}
			builder.WriteString(item.Text)
		}
	}
	if builder.Len() == 0 && payload.Result.StructuredContent != nil {
		encoded, _ := json.Marshal(payload.Result.StructuredContent)
		builder.Write(encoded)
	}
	text := builder.String()
	truncated := false
	if len(text) > 32*1024 {
		text = text[:32*1024] + "\n[Tool result truncated]"
		truncated = true
	}
	return ToolCallResult{Text: text, Truncated: truncated}, nil
}

func timeout(milliseconds int) time.Duration {
	if milliseconds <= 0 {
		return 30 * time.Second
	}
	return time.Duration(milliseconds) * time.Millisecond
}

type boundedBuffer struct {
	buf   bytes.Buffer
	limit int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		return len(p), nil
	}
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.buf.Write(p[:remaining])
		return len(p), nil
	}
	_, _ = b.buf.Write(p)
	return len(p), nil
}
