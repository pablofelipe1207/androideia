package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenCodeZenProvider implementsa el provider para OpenCode Zen, el
// gateway de modelos hospedado por OpenCode (https://opencode.ai/zen).
// Usa el endpoint OpenAI-compatible /v1/chat/completions para chat y
// /v1/embeddings para embeddings, lo que le da máxima portabilidad
// con el ecosistema OpenAI.
//
// Detalles de diseño:
//
//   - El tier free de OpenCode Zen NO requiere API key. Si `apiKey`
//     está vacío, no se envía el header Authorization. Si el usuario
//     tiene una key (login con `opencode auth login --provider zen`),
//     se envía como `Authorization: Bearer <key>`.
//
//   - IsAvailable() hace GET a /v1/models, que es público y devuelve
//     el catálogo completo de modelos disponibles. No consume quota.
//
//   - El `BaseURL` default es https://opencode.ai/zen/v1 pero es
//     configurable para self-hosted gateways compatibles.
//
//   - Para embeddings: hoy OpenCode Zen no expone modelos de
//     embedding en su catálogo público, así que Embed() puede fallar
//     con 404/model_not_found. La implementación está en formato
//     OpenAI-compatible de todos modos, así que cuando Zen agregue
//     embeddings funcionará sin cambios.
type OpenCodeZenProvider struct {
	baseURL    string
	model      string
	apiKey     string
	httpClient *http.Client
}

const defaultOpenCodeZenBaseURL = "https://opencode.ai/zen/v1"

// NewOpenCodeZenProvider crea un provider con la configuración por
// defecto. El `apiKey` puede ser vacío para usar el tier free.
func NewOpenCodeZenProvider(model string) *OpenCodeZenProvider {
	return NewOpenCodeZenProviderWithOptions(model, "", defaultOpenCodeZenBaseURL, 120*time.Second)
}

// NewOpenCodeZenProviderWithAPIKey crea un provider con API key
// explícita. Útil cuando el usuario hizo `opencode auth login` y
// guardó la key en algún lado.
func NewOpenCodeZenProviderWithAPIKey(model, apiKey string) *OpenCodeZenProvider {
	return NewOpenCodeZenProviderWithOptions(model, apiKey, defaultOpenCodeZenBaseURL, 120*time.Second)
}

// NewOpenCodeZenProviderWithOptions expone todos los parámetros. Útil
// para tests (p. ej. baseURL apuntando a un mock server) o gateways
// self-hosted compatibles.
func NewOpenCodeZenProviderWithOptions(model, apiKey, baseURL string, timeout time.Duration) *OpenCodeZenProvider {
	if baseURL == "" {
		baseURL = defaultOpenCodeZenBaseURL
	}
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	// Trim trailing slash para que concatenar "/chat/completions" no
	// genere "//".
	for len(baseURL) > 0 && baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}
	return &OpenCodeZenProvider{
		baseURL:    baseURL,
		model:      model,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// IsAvailable verifica que el provider responde. Lo hace contra
// /v1/models, que es público y barato.
func (z *OpenCodeZenProvider) IsAvailable() bool {
	resp, err := z.httpClient.Get(z.baseURL + "/models")
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK
}

// ListModels devuelve los IDs de los modelos disponibles en el
// catálogo público de OpenCode Zen. Útil para que el usuario vea
// qué modelos puede elegir.
type OpenCodeZenModelsResponse struct {
	Object string                  `json:"object"`
	Data   []OpenCodeZenModelEntry `json:"data"`
}

type OpenCodeZenModelEntry struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

func (z *OpenCodeZenProvider) ListModels() ([]string, error) {
	resp, err := z.httpClient.Get(z.baseURL + "/models")
	if err != nil {
		return nil, fmt.Errorf("error contacting OpenCode Zen at %s: %w", z.baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenCode Zen returned status %d: %s", resp.StatusCode, string(body))
	}

	var out OpenCodeZenModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("error decoding OpenCode Zen response: %w", err)
	}

	names := make([]string, 0, len(out.Data))
	for _, m := range out.Data {
		if m.ID != "" {
			names = append(names, m.ID)
		}
	}
	return names, nil
}

// Chat envía un request a /v1/chat/completions (formato OpenAI) y
// devuelve la respuesta. Mapea tool_calls al formato unificado que
// ya entiende el agente.
func (z *OpenCodeZenProvider) Chat(messages []Message, tools []Tool) (*ChatResponse, error) {
	request := map[string]interface{}{
		"model":    z.model,
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

	req, err := http.NewRequest(http.MethodPost, z.baseURL+"/chat/completions", bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if z.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+z.apiKey)
	}

	resp, err := z.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling OpenCode Zen: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenCode Zen returned status %d: %s", resp.StatusCode, truncate(string(body), 500))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %w", err)
	}

	return &chatResp, nil
}

// EmbeddingRequest es el body de POST /v1/embeddings (formato OpenAI).
// `Input` puede ser un string o []string según la spec; acá usamos
// string para mantener paridad con Ollama.
type EmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// EmbeddingResponse es la respuesta de /v1/embeddings. Devolvemos
// un único vector de floats (no `[][]float64`) porque la API
// Ollama-compatible y nuestro storage usan un solo embedding por
// símbolo. Si el response trae un array, tomamos el primero.
type EmbeddingResponse struct {
	Object string             `json:"object"`
	Data   []EmbeddingDataOne `json:"data"`
	Model  string             `json:"model"`
}

type EmbeddingDataOne struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

// Embed devuelve el embedding de un texto. Envía a /v1/embeddings en
// formato OpenAI.
//
// IMPORTANTE: a la fecha, OpenCode Zen no expone modelos de embedding
// en su catálogo público (https://opencode.ai/zen/v1/models solo
// lista modelos de chat). Si `z.model` no es un modelo de embedding,
// esta llamada va a devolver 400/404. La implementación queda lista
// para cuando Zen agregue embeddings.
func (z *OpenCodeZenProvider) Embed(text string) ([]float32, error) {
	request := EmbeddingRequest{Model: z.model, Input: text}
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, z.baseURL+"/embeddings", bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if z.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+z.apiKey)
	}

	resp, err := z.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling OpenCode Zen embeddings: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"OpenCode Zen embeddings returned status %d: %s\n"+
				"Nota: a la fecha, OpenCode Zen no expone modelos de embedding "+
				"en su catálogo (ver https://opencode.ai/zen/v1/models). "+
				"Si necesitás embeddings, usá Ollama local con `provider: ollama` "+
				"o esperá a que OpenCode agregue esta capacidad.",
			resp.StatusCode, truncate(string(body), 500),
		)
	}

	var embResp EmbeddingResponse
	if err := json.Unmarshal(body, &embResp); err != nil {
		return nil, fmt.Errorf("error unmarshaling embedding response: %w", err)
	}
	if len(embResp.Data) == 0 {
		return nil, fmt.Errorf("OpenCode Zen embeddings returned no data")
	}
	return embResp.Data[0].Embedding, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
