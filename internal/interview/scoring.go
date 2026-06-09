package interview

import (
	"fmt"
	"strings"
)

// ScoreResult representa el resultado de una entrevista.
type ScoreResult struct {
	TotalQuestions    int
	CorrectAnswers    int
	IncorrectAnswers  int
	Percentage        float64
	CategoryScores    map[Category]*CategoryScore
	WeakAreas         []string
	StrongAreas       []string
}

// CategoryScore guarda el desempeño por categoría.
type CategoryScore struct {
	Category   Category
	Total      int
	Correct    int
	Percentage float64
}

// InterviewResponse representa la respuesta del usuario a una pregunta.
type InterviewResponse struct {
	QuestionID string
	Question   Question
	Answer     int
	Correct    bool
}

// NewScoreResult crea un nuevo ScoreResult.
func NewScoreResult() *ScoreResult {
	return &ScoreResult{
		CategoryScores: make(map[Category]*CategoryScore),
	}
}

// AddResponse agrega una respuesta al resultado.
func (s *ScoreResult) AddResponse(response InterviewResponse) {
	s.TotalQuestions++

	if response.Correct {
		s.CorrectAnswers++
	} else {
		s.IncorrectAnswers++
	}

	// Actualizar score de categoría
	cat := response.Question.Category
	if _, exists := s.CategoryScores[cat]; !exists {
		s.CategoryScores[cat] = &CategoryScore{
			Category: cat,
		}
	}
	s.CategoryScores[cat].Total++
	if response.Correct {
		s.CategoryScores[cat].Correct++
	}

	// Calcular porcentajes
	if s.TotalQuestions > 0 {
		s.Percentage = float64(s.CorrectAnswers) / float64(s.TotalQuestions) * 100
	}
	for _, cs := range s.CategoryScores {
		if cs.Total > 0 {
			cs.Percentage = float64(cs.Correct) / float64(cs.Total) * 100
		}
	}
}

// Calculate finaliza el cálculo de scores.
func (s *ScoreResult) Calculate() {
	for _, cs := range s.CategoryScores {
		if cs.Total > 0 {
			cs.Percentage = float64(cs.Correct) / float64(cs.Total) * 100
		}
	}

	// Determinar áreas débiles y fuertes
	for cat, cs := range s.CategoryScores {
		if cs.Percentage < 60 {
			s.WeakAreas = append(s.WeakAreas, string(cat))
		} else if cs.Percentage >= 80 {
			s.StrongAreas = append(s.StrongAreas, string(cat))
		}
	}
}

// GetGrade devuelve una calificación basada en el porcentaje.
func (s *ScoreResult) GetGrade() string {
	switch {
	case s.Percentage >= 90:
		return "A+"
	case s.Percentage >= 80:
		return "A"
	case s.Percentage >= 70:
		return "B"
	case s.Percentage >= 60:
		return "C"
	case s.Percentage >= 50:
		return "D"
	default:
		return "F"
	}
}

// GetFeedback genera feedback personalizado.
func (s *ScoreResult) GetFeedback() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("\n%s\n", strings.Repeat("=", 60)))
	sb.WriteString("  RESULTADO DE LA ENTREVISTA\n")
	sb.WriteString(fmt.Sprintf("%s\n\n", strings.Repeat("=", 60)))

	sb.WriteString(fmt.Sprintf("  Puntuación: %d/%d (%.1f%%) - Calificación: %s\n",
		s.CorrectAnswers, s.TotalQuestions, s.Percentage, s.GetGrade()))

	sb.WriteString(fmt.Sprintf("\n  Correctas: %d | Incorrectas: %d\n",
		s.CorrectAnswers, s.IncorrectAnswers))

	// Scores por categoría
	if len(s.CategoryScores) > 0 {
		sb.WriteString(fmt.Sprintf("\n%s\n", strings.Repeat("-", 60)))
		sb.WriteString("  DESEMPEÑO POR CATEGORÍA:\n")
		sb.WriteString(fmt.Sprintf("%s\n", strings.Repeat("-", 60)))

		for _, cs := range s.CategoryScores {
			bar := strings.Repeat("█", int(cs.Percentage/10))
			empty := strings.Repeat("░", 10-len(bar))
			sb.WriteString(fmt.Sprintf("  %-15s %s%s %.0f%%\n",
				cs.Category, bar, empty, cs.Percentage))
		}
	}

	// Áreas débiles
	if len(s.WeakAreas) > 0 {
		sb.WriteString(fmt.Sprintf("\n%s\n", strings.Repeat("-", 60)))
		sb.WriteString("  ÁREAS DE MEJORA:\n")
		sb.WriteString(fmt.Sprintf("%s\n", strings.Repeat("-", 60)))
		for _, area := range s.WeakAreas {
			sb.WriteString(fmt.Sprintf("  • %s\n", area))
		}
		sb.WriteString("\n  Recomendación: Practica más preguntas de estas categorías.\n")
	}

	// Áreas fuertes
	if len(s.StrongAreas) > 0 {
		sb.WriteString(fmt.Sprintf("\n%s\n", strings.Repeat("-", 60)))
		sb.WriteString("  TUS FORTALEZAS:\n")
		sb.WriteString(fmt.Sprintf("%s\n", strings.Repeat("-", 60)))
		for _, area := range s.StrongAreas {
			sb.WriteString(fmt.Sprintf("  ✓ %s\n", area))
		}
	}

	sb.WriteString(fmt.Sprintf("\n%s\n", strings.Repeat("=", 60)))

	return sb.String()
}

// GetSummary retorna un resumen corto para persistencia.
func (s *ScoreResult) GetSummary() string {
	return fmt.Sprintf("Score: %d/%d (%.1f%%) - Grade: %s",
		s.CorrectAnswers, s.TotalQuestions, s.Percentage, s.GetGrade())
}
