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
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
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
	return NewOllamaProviderWithTimeout(baseURL, model, 120*time.Second)
}

// NewOllamaProviderWithTimeout crea un provider con un timeout
// personalizado por petición. Útil para modelos lentos (7B+ en CPU) o
// contextos largos.
func NewOllamaProviderWithTimeout(baseURL, model string, timeout time.Duration) *OllamaProvider {
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &OllamaProvider{
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: timeout,
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
	// Ollama espera `tool_calls[i].function.arguments` como OBJETO JSON,
	// no como string (que es lo que usa OpenAI). Si los messages vienen
	// con `arguments` como string (p. ej. cuando el extractor de
	// fallback produce tool calls a partir de texto), los convertimos
	// a objeto para evitar el 400
	// "Value looks like object, but can't find closing '}' symbol".
	normalized := normalizeMessagesForOllama(messages)

	request := map[string]interface{}{
		"model":    o.model,
		"messages": normalized,
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
	defer func() { _ = resp.Body.Close() }()

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

	// Handle Ollama's response format (message field at the top level
	// instead of an OpenAI-style `choices` array). We wrap it so the
	// rest of the agent code can use a uniform shape.
	if len(chatResp.Choices) == 0 {
		if chatResp.Message.Content != "" || len(chatResp.Message.ToolCalls) > 0 {
			chatResp.Choices = []Choice{
				{Message: chatResp.Message},
			}
		}
	} else {
		// En algunas versiones de Ollama el `message.tool_calls` viene
		// tanto en el top-level como dentro de `choices[0]`. Si el
		// `choices[0].message` no tiene tool_calls pero el top-level sí,
		// copiamos.
		if len(chatResp.Choices[0].Message.ToolCalls) == 0 && len(chatResp.Message.ToolCalls) > 0 {
			chatResp.Choices[0].Message.ToolCalls = chatResp.Message.ToolCalls
		}
	}

	return &chatResp, nil
}

// normalizeMessagesForOllama prepara los mensajes para la API de Ollama:
//
//  1. Los `tool_calls[i].function.arguments` que sean string JSON se
//     decodifican a `map[string]interface{}` para que se serialicen como
//     objeto y Ollama no rechace la petición.
//
//  2. Los mensajes con `role: "tool"` no llevan `tool_call_id` (Ollama
//     no lo espera; es un campo de OpenAI). Lo dejamos porque Ollama
//     lo ignora silenciosamente, pero por si acaso lo limpiamos.
//
// Devuelve un slice nuevo; no muta el input.
func normalizeMessagesForOllama(in []Message) []Message {
	out := make([]Message, len(in))
	copy(out, in)
	for i := range out {
		if len(out[i].ToolCalls) == 0 {
			continue
		}
		for j, tc := range out[i].ToolCalls {
			switch v := tc.Function.Arguments.(type) {
			case string:
				var m map[string]interface{}
				if err := json.Unmarshal([]byte(v), &m); err == nil && m != nil {
					out[i].ToolCalls[j].Function.Arguments = m
				} else {
					out[i].ToolCalls[j].Function.Arguments = map[string]interface{}{}
				}
			case map[string]interface{}:
				// Ya es un objeto; nada que hacer.
			case nil:
				out[i].ToolCalls[j].Function.Arguments = map[string]interface{}{}
			default:
				// Tipo inesperado (p. ej. json.RawMessage). Lo
				// re-serializamos para garantizar que la salida es un
				// objeto JSON estándar.
				b, err := json.Marshal(v)
				if err == nil {
					var m map[string]interface{}
					if json.Unmarshal(b, &m) == nil {
						out[i].ToolCalls[j].Function.Arguments = m
					}
				}
			}
		}
	}
	return out
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
