package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type OpenAIClient struct {
	httpClient *http.Client
}

type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type StreamRequest struct {
	Provider Provider
	APIKey   string
	Model    string
	Messages []ChatMessage
	Tools    []ChatTool
}

type ChatTool struct {
	Type     string           `json:"type"`
	Function ChatToolFunction `json:"function"`
}

type ChatToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type StreamEvent struct {
	Type      string
	Content   string
	Reasoning string
	ToolCall  *ToolCall
	Usage     *Usage
	Error     error
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func NewOpenAIClient() *OpenAIClient {
	return &OpenAIClient{httpClient: &http.Client{}}
}

func (c *OpenAIClient) ListModels(ctx context.Context, provider Provider, apiKey string) ([]string, error) {
	endpoint, err := joinProviderURL(provider.BaseURL, "/v1/models")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout(provider.RequestTimeoutMS))
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	applyHeaders(req, provider, apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, providerHTTPError(resp)
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&payload); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(payload.Data))
	for _, model := range payload.Data {
		if strings.TrimSpace(model.ID) != "" {
			models = append(models, model.ID)
		}
	}
	return models, nil
}

func (c *OpenAIClient) StreamChat(ctx context.Context, request StreamRequest) (<-chan StreamEvent, error) {
	if strings.TrimSpace(request.Model) == "" {
		return nil, errors.New("model is required")
	}
	endpoint, err := joinProviderURL(request.Provider.BaseURL, "/v1/chat/completions")
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"model":    request.Model,
		"messages": request.Messages,
		"stream":   true,
		"stream_options": map[string]bool{
			"include_usage": true,
		},
	}
	if len(request.Tools) > 0 {
		body["tools"] = request.Tools
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout(request.Provider.RequestTimeoutMS))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		cancel()
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	applyHeaders(req, request.Provider, request.APIKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		cancel()
		return nil, providerHTTPError(resp)
	}
	events := make(chan StreamEvent)
	go func() {
		defer cancel()
		defer resp.Body.Close()
		defer close(events)
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "[DONE]" {
				events <- StreamEvent{Type: "run_completed"}
				return
			}
			for _, event := range ParseStreamChunk([]byte(payload)) {
				select {
				case <-ctx.Done():
					events <- StreamEvent{Type: "run_cancelled", Error: ctx.Err()}
					return
				case events <- event:
				}
			}
		}
		if err := scanner.Err(); err != nil {
			events <- StreamEvent{Type: "run_failed", Error: err}
		}
	}()
	return events, nil
}

func ParseStreamChunk(payload []byte) []StreamEvent {
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content   string `json:"content"`
				Reasoning string `json:"reasoning_content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"delta"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage *Usage `json:"usage"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(payload, &chunk); err != nil {
		return []StreamEvent{{Type: "run_failed", Error: err}}
	}
	if chunk.Error != nil {
		return []StreamEvent{{Type: "run_failed", Error: errors.New(chunk.Error.Message)}}
	}
	var events []StreamEvent
	if chunk.Usage != nil {
		events = append(events, StreamEvent{Type: "usage", Usage: chunk.Usage})
	}
	for _, choice := range chunk.Choices {
		if choice.Delta.Content != "" {
			events = append(events, StreamEvent{Type: "content_delta", Content: choice.Delta.Content})
		}
		if choice.Delta.Reasoning != "" {
			events = append(events, StreamEvent{Type: "reasoning_delta", Reasoning: choice.Delta.Reasoning})
		}
		for _, tool := range choice.Delta.ToolCalls {
			events = append(events, StreamEvent{
				Type: "tool_call_delta",
				ToolCall: &ToolCall{
					ID:   tool.ID,
					Type: tool.Type,
					Function: ToolCallFunction{
						Name:      tool.Function.Name,
						Arguments: tool.Function.Arguments,
					},
				},
			})
		}
		if choice.FinishReason == "tool_calls" {
			events = append(events, StreamEvent{Type: "tool_call_ready"})
		}
	}
	return events
}

func applyHeaders(req *http.Request, provider Provider, apiKey string) {
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	if provider.Organization != "" {
		req.Header.Set("OpenAI-Organization", provider.Organization)
	}
	if provider.Project != "" {
		req.Header.Set("OpenAI-Project", provider.Project)
	}
	for key, value := range provider.CustomHeaders {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}
}

func joinProviderURL(baseURL string, path string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("provider base URL must use http or https")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	return parsed.String(), nil
}

func providerHTTPError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = resp.Status
	}
	return fmt.Errorf("provider returned %s: %s", resp.Status, message)
}

func timeout(milliseconds int) time.Duration {
	if milliseconds <= 0 {
		return 60 * time.Second
	}
	return time.Duration(milliseconds) * time.Millisecond
}
