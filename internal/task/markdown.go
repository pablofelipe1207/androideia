package task

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// MarkdownTask representa una tarea extraída de un archivo .md.
type MarkdownTask struct {
	Line         int    // Número de línea en el archivo (0-indexed)
	Title        string // Título de la tarea (texto después del checkbox)
	Completed    bool   // true si ya está marcada como [x]
	OriginalLine string // Línea original completa para reemplazar
}

// checkboxRegex matches lines like "- [ ] Task" or "- [x] Task" or "* [ ] Task"
var checkboxRegex = regexp.MustCompile(`^(\s*[-*]\s*\[)([ xX])(\]\s*)(.+)$`)

// ParseMarkdownFile lee un archivo .md y extrae las tareas con checkboxes.
func ParseMarkdownFile(filePath string) ([]*MarkdownTask, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening markdown file: %w", err)
	}
	defer file.Close()

	var tasks []*MarkdownTask
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()
		matches := checkboxRegex.FindStringSubmatch(line)

		if len(matches) >= 5 {
			completed := strings.ToLower(matches[2]) == "x"
			tasks = append(tasks, &MarkdownTask{
				Line:         lineNum,
				Title:        strings.TrimSpace(matches[4]),
				Completed:    completed,
				OriginalLine: line,
			})
		}
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading markdown file: %w", err)
	}

	return tasks, nil
}

// GetPendingTasks retorna solo las tareas no completadas.
func GetPendingTasks(tasks []*MarkdownTask) []*MarkdownTask {
	var pending []*MarkdownTask
	for _, t := range tasks {
		if !t.Completed {
			pending = append(pending, t)
		}
	}
	return pending
}

// MarkTaskCompleted lee el archivo, marca la tarea en la línea especificada
// como completada [x], y guarda los cambios.
func MarkTaskCompleted(filePath string, lineNum int) error {
	lines, err := readLines(filePath)
	if err != nil {
		return err
	}

	if lineNum < 0 || lineNum >= len(lines) {
		return fmt.Errorf("line number %d out of range (file has %d lines)", lineNum, len(lines))
	}

	line := lines[lineNum]
	matches := checkboxRegex.FindStringSubmatch(line)

	if len(matches) < 5 {
		return fmt.Errorf("line %d is not a checkbox task: %s", lineNum, line)
	}

	// Reemplazar [ ] o [X] por [x]
	lines[lineNum] = matches[1] + "x" + matches[3] + matches[4]

	return writeLines(filePath, lines)
}

// MarkTaskError lee el archivo, agrega un indicador de error a la tarea
// en la línea especificada, y guarda los cambios.
func MarkTaskError(filePath string, lineNum int, errMsg string) error {
	lines, err := readLines(filePath)
	if err != nil {
		return err
	}

	if lineNum < 0 || lineNum >= len(lines) {
		return fmt.Errorf("line number %d out of range (file has %d lines)", lineNum, len(lines))
	}

	line := lines[lineNum]
	matches := checkboxRegex.FindStringSubmatch(line)

	if len(matches) < 5 {
		return fmt.Errorf("line %d is not a checkbox task: %s", lineNum, line)
	}

	// Mantener el checkbox como [ ] pero agregar comentario de error
	errorComment := fmt.Sprintf("<!-- ERROR: %s -->", errMsg)
	lines[lineNum] = line + " " + errorComment

	return writeLines(filePath, lines)
}

// readLines lee todas las líneas de un archivo.
func readLines(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// writeLines escribe líneas de vuelta al archivo.
func writeLines(filePath string, lines []string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for i, line := range lines {
		if i > 0 {
			if _, err := writer.WriteString("\n"); err != nil {
				return err
			}
		}
		if _, err := writer.WriteString(line); err != nil {
			return err
		}
	}

	return writer.Flush()
}

// Summary retorna un resumen del estado de las tareas.
func Summary(tasks []*MarkdownTask) (pending, completed int) {
	for _, t := range tasks {
		if t.Completed {
			completed++
		} else {
			pending++
		}
	}
	return
}
