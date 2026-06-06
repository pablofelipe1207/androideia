package agent

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/pablofelipe1207/androideia/internal/brain"
	"github.com/pablofelipe1207/androideia/internal/config"
	"github.com/pablofelipe1207/androideia/internal/llm"
)

type Agent struct {
	llm      llm.Provider
	tools    *ToolRegistry
	db       *sql.DB
	config   *config.Config
	messages []llm.Message
	taskStats TaskStats
}

type TaskStats struct {
	FilesCreated  []string
	FilesModified []string
	ToolsUsed     []string
	HasErrors     bool
}

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

func (a *Agent) Run(task string) error {
	fmt.Println("Starting agent loop...")
	fmt.Printf("Task: %s\n\n", task)

	// Reset task stats
	a.taskStats = TaskStats{}

	// Check if LLM is available
	if !a.llm.IsAvailable() {
		return fmt.Errorf("LLM provider is not available. Please check your configuration.")
	}

	// Search for relevant knowledge before starting
	knowledgeContext := a.searchRelevantKnowledge(task)
	
	// Build user message with context
	userMessage := task
	if knowledgeContext != "" {
		userMessage = knowledgeContext + "\n\n---\n\nTask: " + task
	}

	// Add user message
	a.messages = append(a.messages, llm.Message{
		Role:    "user",
		Content: userMessage,
	})

	// Run agent loop
	for {
		fmt.Println("Thinking...")

		// Call LLM
		resp, err := a.llm.Chat(a.messages, a.tools.GetTools())
		if err != nil {
			return fmt.Errorf("error calling LLM: %w", err)
		}

		if len(resp.Choices) == 0 {
			return fmt.Errorf("no response from LLM")
		}

		message := resp.Choices[0].Message
		a.messages = append(a.messages, message)

		// Print response
		if message.Content != "" {
			fmt.Println("\nAgent:")
			fmt.Println(message.Content)
		}

		// Check if agent wants to use tools
		if message.ToolCalls == nil || len(message.ToolCalls) == 0 {
			fmt.Println("\nAgent completed the task.")
			break
		}

		// Process tool calls
		for _, toolCall := range message.ToolCalls {
			fmt.Printf("\nUsing tool: %s\n", toolCall.Function.Name)

			// Parse arguments
			name, args, err := parseToolCall(toolCall)
			if err != nil {
				fmt.Printf("Error parsing tool call: %v\n", err)
				continue
			}

			// Track tool usage
			a.taskStats.ToolsUsed = append(a.taskStats.ToolsUsed, name)

			// Check if this is a write operation that needs approval
			if name == "write_file" {
				if !a.approveWriteOperation(args) {
					fmt.Println("Write operation denied by user.")
					continue
				}
				// Track file operation
				if path, ok := args["path"].(string); ok {
					a.taskStats.FilesCreated = append(a.taskStats.FilesCreated, path)
				}
			}

			// Execute tool
			result, err := a.tools.ExecuteTool(name, args)
			if err != nil {
				fmt.Printf("Error executing tool: %v\n", err)
				result = fmt.Sprintf("Error: %v", err)
				a.taskStats.HasErrors = true
			}

			fmt.Printf("Result: %s\n", result)

			// Add tool result to messages
			a.messages = append(a.messages, llm.Message{
				Role:    "tool",
				Content: result,
			})
		}
	}

	// Store knowledge if task was successful
	if !a.taskStats.HasErrors && len(a.taskStats.FilesCreated) > 0 {
		a.storeTaskKnowledge(task)
	}

	return nil
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
	
	// Show first 10 lines of content
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if i >= 10 {
			fmt.Println("...")
			break
		}
		fmt.Println(line)
	}

	fmt.Printf("\nApprove this write operation? [y/N]: ")
	
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))
	
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

// searchRelevantKnowledge searches the knowledge base for relevant information
// and returns a context string to prepend to the user message
func (a *Agent) searchRelevantKnowledge(task string) string {
	if a.db == nil {
		return ""
	}

	b := brain.NewBrain(a.db)
	
	// Extract key terms from the task for search
	searchTerms := a.extractSearchTerms(task)
	if len(searchTerms) == 0 {
		return ""
	}

	var relevantEntries []string
	seen := make(map[string]bool)

	// Search for each term
	for _, term := range searchTerms {
		entries, err := b.Search(term)
		if err != nil {
			continue
		}

		// Only take promoted entries (verified knowledge)
		for _, entry := range entries {
			if entry.Status == "promoted" && !seen[entry.Title] {
				seen[entry.Title] = true
				relevantEntries = append(relevantEntries, 
					fmt.Sprintf("- %s: %s", entry.Title, truncateContent(entry.Content, 200)))
			}
		}

		// Limit to 5 most relevant entries to keep context fast
		if len(relevantEntries) >= 5 {
			break
		}
	}

	// If no entries found with search, try to get all promoted entries
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

// extractSearchTerms extracts key terms from the task for knowledge search
func (a *Agent) extractSearchTerms(task string) []string {
	var terms []string
	
	// Common Android development terms to search for
	androidTerms := []string{
		"ViewModel", "Repository", "UseCase", "Screen", "Fragment",
		"Activity", "Hilt", "Dagger", "Compose", "Room",
		"Navigation", "Coroutines", "Flow", "LiveData",
		"MVVM", "MVI", "Clean Architecture",
	}
	
	taskLower := strings.ToLower(task)
	
	// Search for Android terms in the task
	for _, term := range androidTerms {
		if strings.Contains(taskLower, strings.ToLower(term)) {
			terms = append(terms, term)
		}
	}
	
	// Also search for any quoted strings or specific class names
	// Simple extraction: look for words that start with uppercase (potential class names)
	words := strings.Fields(task)
	for _, word := range words {
		word = strings.Trim(word, ".,;:!?\"'")
		if len(word) > 3 && strings.HasPrefix(word, strings.ToUpper(string(word[0]))) {
			terms = append(terms, word)
		}
	}
	
	// Limit to 3 terms to keep search fast
	if len(terms) > 3 {
		terms = terms[:3]
	}
	
	return terms
}

// truncateContent truncates content to a maximum length
func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}

// storeTaskKnowledge generates and stores knowledge from a completed task
func (a *Agent) storeTaskKnowledge(task string) {
	if a.db == nil {
		return
	}

	fmt.Println("\n[Brain] Storing task knowledge...")

	// Generate knowledge from the conversation
	knowledge := a.generateKnowledgeFromTask(task)
	if knowledge == nil {
		fmt.Println("[Brain] No knowledge to store")
		return
	}

	// Store in brain
	b := brain.NewBrain(a.db)
	id, err := b.Save(knowledge, false)
	if err != nil {
		fmt.Printf("[Brain] Error storing knowledge: %v\n", err)
		return
	}

	// Auto-promote the entry
	if err := b.Promote(id); err != nil {
		fmt.Printf("[Brain] Error promoting knowledge: %v\n", err)
		return
	}

	fmt.Printf("[Brain] Knowledge stored and promoted (ID: %d)\n", id)
	fmt.Printf("[Brain] Title: %s\n", knowledge.Title)
}

// generateKnowledgeFromTask uses the LLM to extract knowledge from the conversation
func (a *Agent) generateKnowledgeFromTask(task string) *brain.KnowledgeEntry {
	// Build a prompt to extract knowledge
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

	// Add extract prompt to messages
	extractMessages := append([]llm.Message{}, a.messages...)
	extractMessages = append(extractMessages, llm.Message{
		Role:    "user",
		Content: extractPrompt,
	})

	// Call LLM
	resp, err := a.llm.Chat(extractMessages, nil)
	if err != nil {
		fmt.Printf("[Brain] Error calling LLM for knowledge extraction: %v\n", err)
		return nil
	}

	if len(resp.Choices) == 0 {
		return nil
	}

	// Parse the response
	response := resp.Choices[0].Message.Content
	response = strings.TrimSpace(response)

	// Try to parse as JSON
	var knowledgeData struct {
		Title   string `json:"title"`
		Content string `json:"content"`
		Type    string `json:"type"`
		Tags    string `json:"tags"`
	}

	// Simple JSON parsing (handle markdown code blocks)
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

	// Create knowledge entry
	entry := &brain.KnowledgeEntry{
		Type:     knowledgeData.Type,
		Title:    knowledgeData.Title,
		Content:  knowledgeData.Content,
		Tags:     knowledgeData.Tags,
		Status:   "promoted",
		FileRefs: strings.Join(a.taskStats.FilesCreated, ","),
	}

	return entry
}
