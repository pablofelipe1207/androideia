package task

import (
	"fmt"
	"os/exec"
	"strings"
)

// GitHelper proporciona funciones para operaciones git.
type GitHelper struct {
	branchPrefix string
}

// NewGitHelper crea un nuevo GitHelper con el prefijo de branch especificado.
func NewGitHelper(branchPrefix string) *GitHelper {
	if branchPrefix == "" {
		branchPrefix = "task/"
	}
	return &GitHelper{branchPrefix: branchPrefix}
}

// GetCurrentBranch retorna el nombre de la branch actual.
func (g *GitHelper) GetCurrentBranch() (string, error) {
	out, err := runGit("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("error getting current branch: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// CreateBranch crea una nueva branch desde la actual y hace checkout.
func (g *GitHelper) CreateBranch(taskTitle string) (string, error) {
	branchName := g.sanitizeBranchName(taskTitle)

	// Verificar que no exista la branch
	if _, err := runGit("rev-parse", "--verify", branchName); err == nil {
		// La branch ya existe, agregar sufijo numérico
		for i := 1; ; i++ {
			candidate := fmt.Sprintf("%s-%d", branchName, i)
			if _, err := runGit("rev-parse", "--verify", candidate); err != nil {
				branchName = candidate
				break
			}
		}
	}

	// Crear y hacer checkout a la nueva branch
	if _, err := runGit("checkout", "-b", branchName); err != nil {
		return "", fmt.Errorf("error creating branch %s: %w", branchName, err)
	}

	return branchName, nil
}

// CommitAll agrega todos los cambios y crea un commit.
func (g *GitHelper) CommitAll(message string) error {
	// git add -A
	if _, err := runGit("add", "-A"); err != nil {
		return fmt.Errorf("error staging changes: %w", err)
	}

	// Verificar que haya cambios staged
	out, err := runGit("diff", "--cached", "--quiet")
	if err != nil {
		// Si hay cambios, diff --cached --quiet retorna exit code 1
		// Si no hay cambios, retorna exit code 0
		if out == "" {
			// No hay cambios que commitear
			return nil
		}
	}

	// git commit
	if _, err := runGit("commit", "-m", message); err != nil {
		return fmt.Errorf("error committing changes: %w", err)
	}

	return nil
}

// PushBranch hace push de la branch actual al origin.
func (g *GitHelper) PushBranch() error {
	branch, err := g.GetCurrentBranch()
	if err != nil {
		return err
	}

	if _, err := runGit("push", "-u", "origin", branch); err != nil {
		return fmt.Errorf("error pushing branch: %w", err)
	}

	return nil
}

// CreatePR crea un Pull Request usando gh CLI.
func (g *GitHelper) CreatePR(title, body string) (string, error) {
	branch, err := g.GetCurrentBranch()
	if err != nil {
		return "", err
	}

	// Verificar que gh esté disponible
	if _, err := exec.LookPath("gh"); err != nil {
		return "", fmt.Errorf("gh CLI not found. Install it from https://cli.github.com/")
	}

	// Crear PR con gh
	args := []string{
		"pr", "create",
		"--title", title,
		"--body", body,
		"--head", branch,
	}

	out, err := runCommand("gh", args...)
	if err != nil {
		return "", fmt.Errorf("error creating PR: %w", err)
	}

	return strings.TrimSpace(out), nil
}

// sanitizeBranchName convierte un título de tarea en un nombre de branch válido.
func (g *GitHelper) sanitizeBranchName(title string) string {
	// Convertir a minúsculas
	name := strings.ToLower(title)

	// Reemplazar espacios y caracteres especiales por guiones
	replacer := strings.NewReplacer(
		" ", "-",
		"_", "-",
		".", "-",
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "",
		"?", "",
		"\"", "",
		"'", "",
		"<", "",
		">", "",
		"|", "",
		"(", "",
		")", "",
		"[", "",
		"]", "",
		"{", "",
		"}", "",
	)
	name = replacer.Replace(name)

	// Eliminar guiones múltiples
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}

	// Eliminar guiones al inicio y final
	name = strings.Trim(name, "-")

	// Limitar longitud
	if len(name) > 50 {
		name = name[:50]
		name = strings.TrimRight(name, "-")
	}

	// Agregar prefijo
	return g.branchPrefix + name
}

// runGit ejecuta un comando git y retorna la salida.
func runGit(args ...string) (string, error) {
	return runCommand("git", args...)
}

// runCommand ejecuta un comando y retorna la salida.
func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s %s failed: %w\nOutput: %s", name, strings.Join(args, " "), err, string(out))
	}
	return string(out), nil
}

// IsGitRepo verifica si el directorio actual es un repositorio git.
func IsGitRepo() bool {
	_, err := runGit("rev-parse", "--is-inside-work-tree")
	return err == nil
}

// HasChanges verifica si hay cambios sin commitear.
func HasChanges() (bool, error) {
	out, err := runGit("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}
