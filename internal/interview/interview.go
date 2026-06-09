package interview

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pablofelipe1207/androideia/internal/llm"
)

// Interview representa una sesión de entrevista.
type Interview struct {
	db         *sql.DB
	llm        llm.Provider
	score      *ScoreResult
	responses  []InterviewResponse
	questions  []Question
	config     InterviewConfig
}

// InterviewConfig configuración de la entrevista.
type InterviewConfig struct {
	Category   Category
	Difficulty Difficulty
	Count      int
	UseLLM     bool
}

// InterviewSession representa una sesión guardada en la DB.
type InterviewSession struct {
	ID         int64
	Task       string
	Score      string
	Total      int
	Correct    int
	Percentage float64
	Grade      string
	CreatedAt  time.Time
}

// NewInterview crea una nueva sesión de entrevista.
func NewInterview(db *sql.DB, llmProvider llm.Provider, config InterviewConfig) *Interview {
	if config.Count <= 0 {
		config.Count = 10
	}

	return &Interview{
		db:     db,
		llm:    llmProvider,
		score:  NewScoreResult(),
		config: config,
	}
}

// Run ejecuta la sesión de entrevista interactiva.
func (i *Interview) Run() error {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("  ANDROID INTERVIEW - Entrevista Técnica")
	fmt.Println(strings.Repeat("=", 60))

	if i.config.Category != "" {
		fmt.Printf("  Categoría: %s\n", i.config.Category)
	} else {
		fmt.Println("  Categoría: Todas")
	}

	if i.config.Difficulty != "" {
		fmt.Printf("  Dificultad: %s\n", i.config.Difficulty)
	} else {
		fmt.Println("  Dificultad: Todas")
	}

	fmt.Printf("  Preguntas: %d\n", i.config.Count)
	fmt.Println(strings.Repeat("=", 60))

	// Cargar preguntas
	i.questions = GetRandomQuestions(i.config.Category, i.config.Difficulty, i.config.Count)
	if len(i.questions) == 0 {
		return fmt.Errorf("no hay preguntas disponibles para los filtros seleccionados")
	}

	// Ajustar count si hay menos preguntas disponibles
	if i.config.Count > len(i.questions) {
		i.config.Count = len(i.questions)
		fmt.Printf("\n  (Solo hay %d preguntas disponibles con estos filtros)\n", i.config.Count)
	}

	fmt.Println("\n  Instrucciones:")
	fmt.Println("  - Responde con el número de la opción (1-4)")
	fmt.Println("  - Escribe 'q' para salir")
	fmt.Println("  - Escribe 'skip' para saltar una pregunta")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	for idx, question := range i.questions {
		fmt.Printf("\n%s\n", strings.Repeat("-", 60))
		fmt.Printf("  Pregunta %d/%d [%s] [%s]\n",
			idx+1, i.config.Count, question.Category, question.Difficulty)
		fmt.Printf("%s\n\n", strings.Repeat("-", 60))

		fmt.Printf("  %s\n\n", question.Question)

		for i, opt := range question.Options {
			fmt.Printf("    %d) %s\n", i+1, opt)
		}

		fmt.Print("\n  Tu respuesta: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		// Comandos especiales
		if input == "q" || input == "quit" || input == "exit" {
			fmt.Println("\n  Entrevista cancelada.")
			break
		}

		if input == "skip" {
			fmt.Println("  Pregunta saltada.")
			continue
		}

		// Parsear respuesta
		answerIdx, err := strconv.Atoi(input)
		if err != nil || answerIdx < 1 || answerIdx > len(question.Options) {
			fmt.Println("  Respuesta inválida. Usa 1-", len(question.Options))
			idx--
			continue
		}

		// Verificar respuesta
		correct := answerIdx-1 == question.Answer

		response := InterviewResponse{
			QuestionID: question.ID,
			Question:   question,
			Answer:     answerIdx - 1,
			Correct:    correct,
		}

		i.responses = append(i.responses, response)
		i.score.AddResponse(response)

		// Feedback inmediato
		if correct {
			fmt.Printf("\n  ✓ Correcto!\n")
		} else {
			fmt.Printf("\n  ✗ Incorrecto. La respuesta correcta es: %d) %s\n",
				question.Answer+1, question.Options[question.Answer])
		}

		fmt.Printf("\n  Explicación: %s\n", question.Explanation)

		// Si el LLM está disponible y se pide, generar pregunta adicional
		if i.config.UseLLM && i.llm != nil && i.llm.IsAvailable() {
			if shouldGenerate, topic := i.shouldGenerateFollowUp(); shouldGenerate {
				i.generateLLMQuestion(topic)
			}
		}
	}

	// Calcular y mostrar resultado final
	i.score.Calculate()
	fmt.Println(i.score.GetFeedback())

	// Guardar en DB si está disponible
	if i.db != nil {
		if err := i.saveSession(); err != nil {
			fmt.Printf("  (Error guardando sesión: %v)\n", err)
		}
	}

	return nil
}

// shouldGenerateFollowUp determina si se debe generar una pregunta adicional.
func (i *Interview) shouldGenerateFollowUp() (bool, string) {
	if len(i.responses) == 0 {
		return false, ""
	}

	lastResponse := i.responses[len(i.responses)-1]
	if !lastResponse.Correct {
		return true, string(lastResponse.Question.Category)
	}

	// 30% de probabilidad de generar pregunta adicional
	return len(i.responses)%3 == 0, ""
}

// generateLLMQuestion genera una pregunta usando el LLM.
func (i *Interview) generateLLMQuestion(topic string) {
	if i.llm == nil || !i.llm.IsAvailable() {
		return
	}

	prompt := fmt.Sprintf(`Genera una pregunta técnica de Android sobre "%s" para una entrevista de trabajo.
Formato JSON exacto:
{
  "question": "pregunta aquí",
  "options": ["opción 1", "opción 2", "opción 3", "opción 4"],
  "answer": 0,
  "explanation": "explicación de la respuesta correcta"
}

Solo el JSON, sin texto adicional.`, topic)

	messages := []llm.Message{
		{Role: "system", Content: "Eres un experto en Android que genera preguntas de entrevista técnicas."},
		{Role: "user", Content: prompt},
	}

	resp, err := i.llm.Chat(messages, nil)
	if err != nil {
		return
	}

	if len(resp.Choices) == 0 {
		return
	}

	response := resp.Choices[0].Message.Content
	var llmQuestion struct {
		Question    string   `json:"question"`
		Options     []string `json:"options"`
		Answer      int      `json:"answer"`
		Explanation string   `json:"explanation"`
	}

	// Extraer JSON del response
	if idx := strings.Index(response, "{"); idx != -1 {
		if endIdx := strings.LastIndex(response, "}"); endIdx != -1 {
			jsonStr := response[idx : endIdx+1]
			if err := json.Unmarshal([]byte(jsonStr), &llmQuestion); err == nil {
				if len(llmQuestion.Options) == 4 && llmQuestion.Answer >= 0 && llmQuestion.Answer < 4 {
					// Pregunta generada exitosamente
					fmt.Printf("\n  [LLM] Pregunta adicional generada por IA:\n")
					fmt.Printf("  %s\n\n", llmQuestion.Question)
					for j, opt := range llmQuestion.Options {
						fmt.Printf("    %d) %s\n", j+1, opt)
					}
				}
			}
		}
	}
}

// saveSession guarda la sesión en la base de datos.
func (i *Interview) saveSession() error {
	if i.db == nil {
		return nil
	}

	// Crear tabla si no existe
	_, err := i.db.Exec(`
		CREATE TABLE IF NOT EXISTS interview_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task TEXT,
			score TEXT,
			total INTEGER,
			correct INTEGER,
			percentage REAL,
			grade TEXT,
			category TEXT,
			difficulty TEXT,
			created_at INTEGER
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating interview_history table: %w", err)
	}

	// Insertar sesión
	task := fmt.Sprintf("Interview %s/%s", i.config.Category, i.config.Difficulty)
	if i.config.Category == "" {
		task = "Interview General"
	}

	_, err = i.db.Exec(`
		INSERT INTO interview_history (task, score, total, correct, percentage, grade, category, difficulty, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task,
		i.score.GetSummary(),
		i.score.TotalQuestions,
		i.score.CorrectAnswers,
		i.score.Percentage,
		i.score.GetGrade(),
		string(i.config.Category),
		string(i.config.Difficulty),
		time.Now().Unix(),
	)

	if err != nil {
		return fmt.Errorf("error saving interview session: %w", err)
	}

	return nil
}

// GetHistory obtiene el historial de entrevistas.
func GetHistory(db *sql.DB, limit int) ([]InterviewSession, error) {
	if db == nil {
		return nil, fmt.Errorf("database not available")
	}

	// Asegurar que la tabla existe
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS interview_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task TEXT,
			score TEXT,
			total INTEGER,
			correct INTEGER,
			percentage REAL,
			grade TEXT,
			category TEXT,
			difficulty TEXT,
			created_at INTEGER
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("error creating table: %w", err)
	}

	if limit <= 0 {
		limit = 10
	}

	rows, err := db.Query(`
		SELECT id, task, score, total, correct, percentage, grade, created_at
		FROM interview_history
		ORDER BY created_at DESC
		LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("error querying history: %w", err)
	}
	defer rows.Close()

	var sessions []InterviewSession
	for rows.Next() {
		var s InterviewSession
		var createdAt int64
		if err := rows.Scan(&s.ID, &s.Task, &s.Score, &s.Total, &s.Correct, &s.Percentage, &s.Grade, &createdAt); err != nil {
			continue
		}
		s.CreatedAt = time.Unix(createdAt, 0)
		sessions = append(sessions, s)
	}

	return sessions, nil
}

// PrintHistory muestra el historial de entrevistas.
func PrintHistory(db *sql.DB) {
	sessions, err := GetHistory(db, 10)
	if err != nil {
		fmt.Printf("Error obteniendo historial: %v\n", err)
		return
	}

	if len(sessions) == 0 {
		fmt.Println("\n  No hay entrevistas anteriores.")
		return
	}

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("  HISTORIAL DE ENTREVISTAS")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("  %-6s %-25s %-10s %-8s %-6s %s\n", "ID", "Fecha", "Score", "Grade", "%", "Tarea")
	fmt.Println(strings.Repeat("-", 70))

	for _, s := range sessions {
		fmt.Printf("  %-6d %-25s %-10s %-8s %-6.0f %s\n",
			s.ID,
			s.CreatedAt.Format("2006-01-02 15:04"),
			s.Score,
			s.Grade,
			s.Percentage,
			s.Task,
		)
	}

	fmt.Println(strings.Repeat("=", 70))
}
