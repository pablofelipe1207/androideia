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

// OllamaTagsResponse representa la respuesta de GET /api/tags de Ollama.
// Solo nos interesa el nombre de cada modelo.
type OllamaTagsResponse struct {
	Models []OllamaModel `json:"models"`
}

type OllamaModel struct {
	Name string `json:"name"`
}

// ListModels devuelve los nombres de los modelos disponibles en Ollama.
// Devuelve un error si no se puede contactar a Ollama o si la respuesta
// no se puede decodificar.
func (o *OllamaProvider) ListModels() ([]string, error) {
	resp, err := o.httpClient.Get(o.baseURL + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("error contacting Ollama at %s: %w", o.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var tags OllamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("error decoding Ollama response: %w", err)
	}

	names := make([]string, 0, len(tags.Models))
	for _, m := range tags.Models {
		if m.Name != "" {
			names = append(names, m.Name)
		}
	}
	return names, nil
}

// ResolveOllamaModel decide qué modelo usar consultando Ollama.
//
// Reglas:
//   - Si no se puede contactar a Ollama, devuelve configuredModel sin error
//     (dejamos que IsAvailable falle más tarde con un mensaje claro).
//   - Si Ollama tiene exactamente 1 modelo, devuelve ese modelo.
//   - Si el modelo configurado está en la lista, devuelve configuredModel.
//   - Si hay varios modelos y el configurado no está, devuelve un error
//     listando los modelos disponibles.
//
// Devuelve un bool adicional `autoSelected` que es true cuando se eligió
// el único modelo de Ollama (distinto al configurado).
func ResolveOllamaModel(ollamaURL, configuredModel string) (string, bool, error) {
	provider := NewOllamaProvider(ollamaURL, configuredModel)
	models, err := provider.ListModels()
	if err != nil {
		// No se pudo hablar con Ollama: caemos al modelo configurado.
		return configuredModel, false, nil
	}

	if len(models) == 0 {
		return configuredModel, false, fmt.Errorf("Ollama no tiene modelos instalados (ejecuta: ollama pull <modelo>)")
	}

	if len(models) == 1 {
		return models[0], models[0] != configuredModel, nil
	}

	for _, m := range models {
		if m == configuredModel {
			return configuredModel, false, nil
		}
	}

	return "", false, fmt.Errorf(
		"el modelo configurado %q no está disponible en Ollama. Modelos disponibles: %v\nUsa 'androideai config set model <modelo>' o el flag --model",
		configuredModel, models,
	)
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
