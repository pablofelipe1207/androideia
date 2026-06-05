package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolCallFunc `json:"function"`
}

type ToolCallFunc struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments"`
	Index     int         `json:"index,omitempty"`
}

type ChatResponse struct {
	Choices []Choice `json:"choices,omitempty"`
	Message Message  `json:"message,omitempty"`
}

type Choice struct {
	Message Message `json:"message"`
}

type Provider interface {
	Chat(messages []Message, tools []Tool) (*ChatResponse, error)
	IsAvailable() bool
}

type OllamaProvider struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

func NewOllamaProvider(baseURL, model string) *OllamaProvider {
	return &OllamaProvider{
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (o *OllamaProvider) IsAvailable() bool {
	resp, err := o.httpClient.Get(o.baseURL + "/api/tags")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (o *OllamaProvider) Chat(messages []Message, tools []Tool) (*ChatResponse, error) {
	request := map[string]interface{}{
		"model":    o.model,
		"messages": messages,
		"stream":   false,
	}

	if len(tools) > 0 {
		request["tools"] = tools
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	resp, err := o.httpClient.Post(o.baseURL+"/api/chat", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error calling Ollama: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %w", err)
	}

	// Handle Ollama's response format (message field instead of choices)
	if len(chatResp.Choices) == 0 && chatResp.Message.Content != "" {
		chatResp.Choices = []Choice{
			{Message: chatResp.Message},
		}
	}

	return &chatResp, nil
}

type AnthropicProvider struct {
	apiKey  string
	model   string
	client  *http.Client
}

func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	return &AnthropicProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (a *AnthropicProvider) IsAvailable() bool {
	return a.apiKey != ""
}

func (a *AnthropicProvider) Chat(messages []Message, tools []Tool) (*ChatResponse, error) {
	// Stub implementation
	return nil, fmt.Errorf("Anthropic provider not implemented yet")
}

type OpenAIProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

func NewOpenAIProvider(apiKey, model, baseURL string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (o *OpenAIProvider) IsAvailable() bool {
	return o.apiKey != ""
}

func (o *OpenAIProvider) Chat(messages []Message, tools []Tool) (*ChatResponse, error) {
	// Stub implementation
	return nil, fmt.Errorf("OpenAI provider not implemented yet")
}
