package agent

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pablofelipe1207/androideia/internal/brain"
	"github.com/pablofelipe1207/androideia/internal/config"
	"github.com/pablofelipe1207/androideia/internal/llm"
	"github.com/pablofelipe1207/androideia/internal/memory"
)

type Agent struct {
	llm            llm.Provider
	tools          *ToolRegistry
	db             *sql.DB
	config         *config.Config
	messages       []llm.Message
	memory         *memory.Memory
	conversationID int64
	taskStats      TaskStats
}

type TaskStats struct {
	FilesCreated  []string
	FilesModified []string
	ToolsUsed     []string
	HasErrors     bool
}

// NewAgent crea un agente. La memoria persistente se conecta con SetMemory.
func NewAgent(llmProvider llm.Provider, db *sql.DB, cfg *config.Config) *Agent {
	return &Agent{
		llm:   llmProvider,
		tools: NewToolRegistry(db),
		db:    db,
		config: cfg,
		messages: []llm.Message{
			{Role: "system", Content: SystemPrompt},
		},
	}
}

// SetMemory conecta el store de memoria persistente. Si el agente ya tiene
// un conversationID, no se reinicia; sirve para habilitar persistencia
// desde el comando.
func (a *Agent) SetMemory(mem *memory.Memory) {
	a.memory = mem
}

// StartSession crea una nueva conversación para la tarea y la marca como activa.
// Devuelve el ID de la conversación.
func (a *Agent) StartSession(task string) (int64, error) {
	if a.memory == nil {
		return 0, nil
	}
	conv, err := a.memory.CreateConversation(task, "", a.config.Approval, a.config.Provider, a.config.Model)
	if err != nil {
		return 0, err
	}
	a.conversationID = conv.ID

	// Persistir el system prompt como primer mensaje de la conversación
	// para que un resume reconstruya exactamente el mismo contexto.
	if err := a.memory.AppendMessage(a.conversationID, "system", SystemPrompt, nil, "", ""); err != nil {
		return 0, err
	}
	return conv.ID, nil
}

// ResumeSession carga una conversación existente y reconstruye el slice
// de mensajes en memoria. Devuelve la tarea original.
func (a *Agent) ResumeSession(id int64) (string, error) {
	if a.memory == nil {
		return "", fmt.Errorf("memory not configured")
	}
	conv, err := a.memory.GetConversation(id)
	if err != nil {
		return "", err
	}
	if conv.Status == memory.StatusCompleted {
		return "", fmt.Errorf("conversation %d is already marked as completed", id)
	}
	a.conversationID = conv.ID
	stored, err := a.memory.LoadMessages(id)
	if err != nil {
		return "", err
	}
	a.messages = a.memory.ToLLMMessages(stored)
	if err := a.memory.SetStatus(id, memory.StatusActive); err != nil {
		return "", err
	}
	return conv.Task, nil
}

// ConversationID devuelve el ID de la conversación activa (0 si no hay memoria).
func (a *Agent) ConversationID() int64 {
	return a.conversationID
}

// persistMessage guarda un mensaje en la memoria persistente si está habilitada.
func (a *Agent) persistMessage(role, content string, toolCalls []llm.ToolCall, toolCallID, toolName string) {
	if a.memory == nil || a.conversationID == 0 {
		return
	}
	_ = a.memory.AppendMessage(a.conversationID, role, content, toolCalls, toolCallID, toolName)
}

func (a *Agent) Run(task string) error {
	fmt.Println("Starting agent loop...")
	fmt.Printf("Task: %s\n\n", task)

	// Si no hay sesión activa y la memoria está habilitada, arrancar una.
	if a.conversationID == 0 && a.memory != nil {
		convID, err := a.StartSession(task)
		if err != nil {
			return fmt.Errorf("error creating conversation: %w", err)
		}
		fmt.Printf("[Memory] Nueva conversación #%d\n", convID)
	}
	// Marcar la sesión como activa al comenzar (sobrevive a SIGINT).
	if a.memory != nil && a.conversationID != 0 {
		_ = a.memory.SetStatus(a.conversationID, memory.StatusActive)
	}
	// Si estamos reanudando, el task ya está en la conversación; el caller
	// debe pasar el mensaje NUEVO del usuario. Si llega vacío, no añadimos nada.
	if task != "" {
		if err := a.appendUserTurn(task); err != nil {
			return err
		}
	}
	// Si por la razón que sea no hay mensajes del usuario, abortar limpio.
	hasUser := false
	for _, m := range a.messages {
		if m.Role == "user" {
			hasUser = true
			break
		}
	}
	if !hasUser {
		return fmt.Errorf("no user message to process (use --resume <id> with un mensaje or pass a task)")
	}

	// Reset task stats para esta ejecución
	a.taskStats = TaskStats{}

	// Check if LLM is available
	if !a.llm.IsAvailable() {
		return fmt.Errorf("LLM provider is not available. Please check your configuration.")
	}

	// Run agent loop
	const maxTurns = 25 // tope de seguridad para evitar loops infinitos
	for turn := 0; turn < maxTurns; turn++ {
		fmt.Println("Thinking...")

		// Progress indicator: imprime el elapsed time cada 30s para
		// que el usuario sepa que el LLM sigue trabajando (los modelos
		// 7B+ en CPU pueden tardar minutos en generar planes largos).
		progressDone := make(chan struct{})
		go func(start time.Time, turn int) {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-progressDone:
					return
				case <-ticker.C:
					fmt.Printf("... still thinking (turn %d, elapsed: %s) ...\n",
						turn+1, time.Since(start).Round(time.Second))
				}
			}
		}(time.Now(), turn)

		// Call LLM
		resp, err := a.llm.Chat(a.messages, a.tools.GetTools())
		close(progressDone)
		if err != nil {
			a.markInterrupted()
			return fmt.Errorf("error calling LLM: %w\n\nLa sesión quedó guardada como 'interrupted'. Usa 'androideai agent --resume %d' para continuar o 'androideai memory show %d' para revisarla.\nSi el timeout es muy corto, prueba --timeout 600 (10 min) o súbelo en la config con 'androideai config set timeout 900'.",
				err, a.conversationID, a.conversationID)
		}

		if len(resp.Choices) == 0 {
			a.markInterrupted()
			return fmt.Errorf("no response from LLM")
		}

		message := resp.Choices[0].Message

		// Fallback: algunos modelos (o modelos mal entrenados / prompts
		// mal interpretados) emiten las tool calls como JSON en el campo
		// `content` en vez de usar el mecanismo nativo. Si no hay
		// tool_calls nativos, intentamos extraerlos del texto para no
		// dejar la tarea a medias.
		if len(message.ToolCalls) == 0 && message.Content != "" {
			extracted := extractToolCallsFromContent(message.Content)
			if len(extracted) > 0 {
				fmt.Printf("[Fallback] Extraídas %d tool call(s) del texto del agente.\n", len(extracted))
				message.ToolCalls = extracted
			}
		}

		a.messages = append(a.messages, message)
		a.persistMessage(message.Role, message.Content, message.ToolCalls, "", "")

		// Print response
		if message.Content != "" {
			fmt.Println("\nAgent:")
			fmt.Println(message.Content)
		}

		// Si el agente llama a herramientas, procesamos y seguimos.
		if message.ToolCalls != nil && len(message.ToolCalls) > 0 {
			anyDenied := false
			for _, toolCall := range message.ToolCalls {
				fmt.Printf("\nUsing tool: %s\n", toolCall.Function.Name)

				// Parse arguments
				name, args, err := parseToolCall(toolCall)
				if err != nil {
					fmt.Printf("Error parsing tool call: %v\n", err)
					a.persistMessage("tool", fmt.Sprintf("Error: %v", err), nil, toolCall.ID, name)
					a.messages = append(a.messages, llm.Message{
						Role:       "tool",
						Content:    fmt.Sprintf("Error: %v", err),
						ToolCallID: toolCall.ID,
					})
					continue
				}

				a.taskStats.ToolsUsed = append(a.taskStats.ToolsUsed, name)

				// Gate de aprobación para write_file (modo ask).
				if name == "write_file" {
					if !a.approveWriteOperation(args) {
						fmt.Println("Write operation denied by user.")
						result := "denied by user"
						a.persistMessage("tool", result, nil, toolCall.ID, name)
						a.messages = append(a.messages, llm.Message{
							Role:       "tool",
							Content:    result,
							ToolCallID: toolCall.ID,
						})
						anyDenied = true
						continue
					}
					if path, ok := args["path"].(string); ok {
						a.taskStats.FilesCreated = append(a.taskStats.FilesCreated, path)
					}
				}

				// Si el agente pidió confirmar el plan vía herramienta, la tool ya
				// interactuó con el usuario. Detectamos "denied" en el resultado
				// para cortar el loop con elegancia.
				result, err := a.tools.ExecuteTool(name, args)
				if err != nil {
					fmt.Printf("Error executing tool: %v\n", err)
					result = fmt.Sprintf("Error: %v", err)
					a.taskStats.HasErrors = true
				}

				fmt.Printf("Result: %s\n", result)
				a.persistMessage("tool", result, nil, toolCall.ID, name)
				a.messages = append(a.messages, llm.Message{
					Role:       "tool",
					Content:    result,
					ToolCallID: toolCall.ID,
				})

				if name == "confirm_plan" && strings.HasPrefix(strings.TrimSpace(strings.ToLower(result)), "denied") {
					anyDenied = true
				}
			}

			if anyDenied {
				fmt.Println("\n[Agent] Plan rejected by user. Conversation remains open for follow-up.")
				a.markInterrupted()
				return nil
			}
			continue
		}

		// No hubo tool calls. Antes de cerrar, ver si el texto plano del
		// agente parece una petición de confirmación. Si lo es, preguntamos.
		if a.looksLikeConfirmationRequest(message.Content) {
			fmt.Println("\n[Detectado] El agente está pidiendo confirmación en texto plano.")
			response := a.promptUserForConfirmation()
			a.persistMessage("user", response, nil, "", "")

			// Empujar la respuesta como mensaje user para que el LLM reaccione.
			a.messages = append(a.messages, llm.Message{
				Role:    "user",
				Content: response,
			})

			lower := strings.ToLower(strings.TrimSpace(response))
			if strings.HasPrefix(lower, "denied") || lower == "n" || lower == "no" {
				fmt.Println("\n[Agent] Plan rejected by user. Conversation remains open for follow-up.")
				a.markInterrupted()
				return nil
			}
			// Aprobado o con feedback: el agente continuará en la próxima vuelta.
			continue
		}

		// No es una petición de confirmación: el agente no tiene
		// más tool calls que ejecutar. En vez de declarar la tarea
		// como completada por su cuenta, preguntamos al usuario.
		completed := a.promptTaskCompletion()
		if completed {
			a.markCompleted()
		} else {
			// Mantener la sesión como active: el usuario puede
			// continuarla con `androideai agent --resume <id> "..."`
			// o cerrarla con `androideai memory delete <id>`.
			a.markInterrupted()
		}
		return nil
	}

	fmt.Println("\n[Agent] Reached max turns. Stopping loop and saving session.")
	a.markInterrupted()
	return nil
}

// appendUserTurn añade un mensaje de usuario, enriqueciéndolo opcionalmente
// con conocimiento relevante de brain. Persiste el resultado.
func (a *Agent) appendUserTurn(task string) error {
	knowledgeContext := a.searchRelevantKnowledge(task)
	userMessage := task
	if knowledgeContext != "" {
		userMessage = knowledgeContext + "\n\n---\n\nTask: " + task
	}
	a.messages = append(a.messages, llm.Message{
		Role:    "user",
		Content: userMessage,
	})
	a.persistMessage("user", userMessage, nil, "", "")
	return nil
}

func (a *Agent) markCompleted() {
	if a.memory == nil || a.conversationID == 0 {
		return
	}
	if !a.taskStats.HasErrors && len(a.taskStats.FilesCreated) > 0 {
		_ = a.memory.SetStatus(a.conversationID, memory.StatusCompleted)
		a.storeTaskKnowledgeIfReady()
		return
	}
	_ = a.memory.SetStatus(a.conversationID, memory.StatusCompleted)
}

func (a *Agent) markInterrupted() {
	if a.memory == nil || a.conversationID == 0 {
		return
	}
	_ = a.memory.SetStatus(a.conversationID, memory.StatusInterrupted)
}

func (a *Agent) approveWriteOperation(args map[string]interface{}) bool {
	if a.config.Approval == "auto" {
		return true
	}

	path, _ := args["path"].(string)
	content, _ := args["content"].(string)

	fmt.Printf("\n=== Write Operation ===\n")
	fmt.Printf("File: %s\n", path)
	fmt.Printf("Content preview:\n")

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if i >= 10 {
			fmt.Println("...")
			break
		}
		fmt.Println(line)
	}

	fmt.Printf("\nApprove this write operation? [y/N]: ")

	response := readUserResponse()
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

func (a *Agent) GetConversationHistory() []llm.Message {
	return a.messages
}

func (a *Agent) ClearConversation() {
	a.messages = []llm.Message{
		{Role: "system", Content: SystemPrompt},
	}
}

// looksLikeConfirmationRequest devuelve true si el contenido del agente
// parece pedir confirmación al usuario antes de continuar. Cubre español
// e inglés.
func (a *Agent) looksLikeConfirmationRequest(content string) bool {
	if content == "" {
		return false
	}
	lc := strings.ToLower(content)
	patterns := []string{
		"please confirm",
		"do you confirm",
		"do you want me to",
		"would you like me to",
		"shall i proceed",
		"should i proceed",
		"may i proceed",
		"can i proceed",
		"do you approve",
		"¿confirmas",
		"¿puedes confirmar",
		"¿procedo",
		"¿debo continuar",
		"¿quieres que",
		"deseas que",
		"should i continue",
		"please approve",
		"please review",
	}
	for _, p := range patterns {
		if strings.Contains(lc, p) {
			return true
		}
	}
	// Patrón final: termina con signo de interrogación y menciona
	// "confirm" o "proceed" o "continue".
	if (strings.HasSuffix(strings.TrimSpace(content), "?") || strings.HasSuffix(strings.TrimSpace(content), "?")) {
		for _, kw := range []string{"confirm", "proceed", "continue", "approve"} {
			if strings.Contains(lc, kw) {
				return true
			}
		}
	}
	return false
}

// promptUserForConfirmation muestra un prompt de aprobación y devuelve la
// respuesta que se inyectará al LLM. La respuesta es siempre un texto
// natural que el LLM puede entender ("approved", "denied", o feedback).
func (a *Agent) promptUserForConfirmation() string {
	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("  El agente quiere continuar. ¿Apruebas?")
	fmt.Println("  [y=aprobar / n=rechazar / feedback libre]")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Print("> ")

	response := readUserResponse()
	response = strings.TrimSpace(response)
	lower := strings.ToLower(response)

	switch {
	case lower == "" || lower == "n" || lower == "no":
		return "denied by user"
	case lower == "y" || lower == "yes" || lower == "s" || lower == "si" || lower == "sí":
		return "approved"
	default:
		// Feedback libre: se inyecta tal cual para que el LLM lo procese.
		return "approved (user feedback: " + response + ")"
	}
}

// promptTaskCompletion pregunta al usuario si la tarea quedó completa.
// Devuelve true sólo si el usuario responde y/yes. Cualquier otra
// respuesta (incluyendo Enter vacío) deja la sesión como `active`
// para que pueda continuarse con `--resume`.
//
// El agente **nunca** debe marcarse a sí mismo como completado: es
// siempre el usuario quien decide.
func (a *Agent) promptTaskCompletion() bool {
	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("  El agente ha terminado su turno.")
	if len(a.taskStats.FilesCreated) > 0 {
		fmt.Printf("  Archivos creados/modificados: %d\n", len(a.taskStats.FilesCreated))
		for _, p := range a.taskStats.FilesCreated {
			fmt.Printf("    - %s\n", p)
		}
	}
	if a.taskStats.HasErrors {
		fmt.Println("  ⚠ Hubo errores durante la ejecución.")
	}
	fmt.Println("  ¿La tarea quedó completada a tu satisfacción?")
	fmt.Println("  [y=marcar como completada / N o Enter=mantener activa para continuar luego]")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Print("> ")

	response := readUserResponse()
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes" || response == "s" || response == "si" || response == "sí"
}

// searchRelevantKnowledge busca en brain entradas promovidas relacionadas
// con la tarea. Se usa para enriquecer el mensaje del usuario.
func (a *Agent) searchRelevantKnowledge(task string) string {
	if a.db == nil {
		return ""
	}

	b := brain.NewBrain(a.db)
	searchTerms := a.extractSearchTerms(task)
	if len(searchTerms) == 0 {
		return ""
	}

	var relevantEntries []string
	seen := make(map[string]bool)

	for _, term := range searchTerms {
		entries, err := b.Search(term)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.Status == "promoted" && !seen[entry.Title] {
				seen[entry.Title] = true
				relevantEntries = append(relevantEntries,
					fmt.Sprintf("- %s: %s", entry.Title, truncateContent(entry.Content, 200)))
			}
		}
		if len(relevantEntries) >= 5 {
			break
		}
	}

	if len(relevantEntries) == 0 {
		entries, err := b.List()
		if err == nil {
			for _, entry := range entries {
				if entry.Status == "promoted" && !seen[entry.Title] {
					seen[entry.Title] = true
					relevantEntries = append(relevantEntries,
						fmt.Sprintf("- %s: %s", entry.Title, truncateContent(entry.Content, 200)))
					if len(relevantEntries) >= 3 {
						break
					}
				}
			}
		}
	}

	if len(relevantEntries) == 0 {
		return ""
	}
	return "## Relevant Project Knowledge\n\n" + strings.Join(relevantEntries, "\n")
}

func (a *Agent) extractSearchTerms(task string) []string {
	var terms []string
	androidTerms := []string{
		"ViewModel", "Repository", "UseCase", "Screen", "Fragment",
		"Activity", "Hilt", "Dagger", "Compose", "Room",
		"Navigation", "Coroutines", "Flow", "LiveData",
		"MVVM", "MVI", "Clean Architecture",
	}
	taskLower := strings.ToLower(task)
	for _, term := range androidTerms {
		if strings.Contains(taskLower, strings.ToLower(term)) {
			terms = append(terms, term)
		}
	}
	words := strings.Fields(task)
	for _, word := range words {
		word = strings.Trim(word, ".,;:!?\"'")
		if len(word) > 3 && strings.HasPrefix(word, strings.ToUpper(string(word[0]))) {
			terms = append(terms, word)
		}
	}
	if len(terms) > 3 {
		terms = terms[:3]
	}
	return terms
}

func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}

// storeTaskKnowledgeIfReady extrae conocimiento útil de la conversación y
// lo guarda en brain. Sólo se invoca si la tarea generó archivos.
func (a *Agent) storeTaskKnowledgeIfReady() {
	if a.db == nil {
		return
	}
	knowledge := a.generateKnowledgeFromTask()
	if knowledge == nil {
		return
	}
	b := brain.NewBrain(a.db)
	id, err := b.Save(knowledge, false)
	if err != nil {
		fmt.Printf("[Brain] Error storing knowledge: %v\n", err)
		return
	}
	if err := b.Promote(id); err != nil {
		fmt.Printf("[Brain] Error promoting knowledge: %v\n", err)
		return
	}
	fmt.Printf("[Brain] Knowledge stored and promoted (ID: %d): %s\n", id, knowledge.Title)
}

func (a *Agent) generateKnowledgeFromTask() *brain.KnowledgeEntry {
	task := ""
	if a.conversationID != 0 && a.memory != nil {
		if c, err := a.memory.GetConversation(a.conversationID); err == nil {
			task = c.Task
		}
	}
	if task == "" {
		return nil
	}
	if len(a.taskStats.FilesCreated) == 0 {
		return nil
	}

	extractPrompt := fmt.Sprintf(`Based on this development task that was completed successfully, extract the key knowledge that was learned or applied.

Task: %s

Files created/modified: %s

Provide a JSON response with:
{
  "title": "Brief title for this knowledge (max 50 chars)",
  "content": "Description of the pattern, decision, or implementation details (max 200 chars)",
  "type": "architecture|pattern|decision|implementation",
  "tags": "comma-separated relevant tags"
}

Only return the JSON, no other text.`, task, strings.Join(a.taskStats.FilesCreated, ", "))

	extractMessages := append([]llm.Message{}, a.messages...)
	extractMessages = append(extractMessages, llm.Message{
		Role:    "user",
		Content: extractPrompt,
	})

	resp, err := a.llm.Chat(extractMessages, nil)
	if err != nil {
		fmt.Printf("[Brain] Error calling LLM for knowledge extraction: %v\n", err)
		return nil
	}
	if len(resp.Choices) == 0 {
		return nil
	}

	response := strings.TrimSpace(resp.Choices[0].Message.Content)
	var knowledgeData struct {
		Title   string `json:"title"`
		Content string `json:"content"`
		Type    string `json:"type"`
		Tags    string `json:"tags"`
	}

	if idx := strings.Index(response, "{"); idx != -1 {
		if endIdx := strings.LastIndex(response, "}"); endIdx != -1 {
			jsonStr := response[idx : endIdx+1]
			if err := json.Unmarshal([]byte(jsonStr), &knowledgeData); err != nil {
				fmt.Printf("[Brain] Error parsing knowledge JSON: %v\n", err)
				return nil
			}
		}
	}

	if knowledgeData.Title == "" {
		return nil
	}
	return &brain.KnowledgeEntry{
		Type:     knowledgeData.Type,
		Title:    knowledgeData.Title,
		Content:  knowledgeData.Content,
		Tags:     knowledgeData.Tags,
		Status:   "promoted",
		FileRefs: strings.Join(a.taskStats.FilesCreated, ","),
	}
}

// storeTaskKnowledge genera y guarda conocimiento tras una tarea exitosa.
// Conserva el nombre original para no romper consumidores externos/tests.
func (a *Agent) storeTaskKnowledge(task string) {
	a.storeTaskKnowledgeIfReady()
}

// extractToolCallsFromContent busca objetos JSON con la forma
//   {"name": "<tool>", "arguments": {...}}
// dentro de un texto y los devuelve como llm.ToolCall. Es tolerante a
// explicaciones en lenguaje natural mezcladas con los JSON, y a JSON
// envuelto en bloques de código markdown ```json ... ```.
//
// Se usa como red de seguridad para modelos que no usan la API nativa
// de tool calling y en su lugar imprimen las llamadas como texto.
func extractToolCallsFromContent(content string) []llm.ToolCall {
	var calls []llm.ToolCall
	for i := 0; i < len(content); i++ {
		if content[i] != '{' {
			continue
		}
		// Encontrar la `}` de cierre respetando strings y escapes.
		depth := 0
		inString := false
		escaped := false
		end := -1
		for j := i; j < len(content); j++ {
			c := content[j]
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				if inString {
					escaped = true
				}
			case '"':
				inString = !inString
			case '{':
				if !inString {
					depth++
				}
			case '}':
				if !inString {
					depth--
					if depth == 0 {
						end = j + 1
					}
				}
			}
			if end != -1 {
				break
			}
		}
		if end == -1 {
			continue
		}
		candidate := content[i:end]

		// Comprobamos que sea un tool call (que tenga `name` y `arguments`).
		var probe struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(candidate), &probe); err != nil {
			i = end - 1
			continue
		}
		if probe.Name == "" {
			i = end - 1
			continue
		}

		// Si `arguments` falta, inicializamos a {} para que el resto
		// del código reciba un objeto JSON válido.
		if len(probe.Arguments) == 0 {
			probe.Arguments = json.RawMessage(`{}`)
		}

		// Decodificamos los arguments a map[string]interface{} para que
		// al serializar el mensaje se emita como OBJETO JSON, no como
		// string. Ollama (a diferencia de OpenAI) exige que
		// `tool_calls[i].function.arguments` sea un objeto; si lo
		// enviamos como string devuelve 400 con
		// "Value looks like object, but can't find closing '}' symbol".
		var argsMap map[string]interface{}
		if err := json.Unmarshal(probe.Arguments, &argsMap); err != nil {
			argsMap = map[string]interface{}{}
		}

		calls = append(calls, llm.ToolCall{
			ID:   fmt.Sprintf("call_text_%d", len(calls)+1),
			Type: "function",
			Function: llm.ToolCallFunc{
				Name:      probe.Name,
				Arguments: argsMap,
			},
		})
		i = end - 1
	}
	return calls
}
