package cmd

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pablofelipe1207/androideia/internal/brain"
	"github.com/pablofelipe1207/androideia/internal/store"
	"github.com/spf13/cobra"
)

var brainCmd = &cobra.Command{
	Use:   "brain",
	Short: "Gestiona la memoria del proyecto",
	Long:  `Operaciones de memoria del proyecto: guardar, buscar, revisar y promover conocimiento.`,
}

var brainSaveCmd = &cobra.Command{
	Use:   "save",
	Short: "Guarda una entrada de conocimiento",
	Long:  `Guarda una nueva entrada de conocimiento en la memoria del proyecto.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get flags
		entryType, _ := cmd.Flags().GetString("type")
		title, _ := cmd.Flags().GetString("title")
		content, _ := cmd.Flags().GetString("content")
		tags, _ := cmd.Flags().GetString("tags")
		fileRefs, _ := cmd.Flags().GetString("file-refs")
		yes, _ := cmd.Flags().GetBool("yes")

		// Validate required fields
		if title == "" {
			return fmt.Errorf("title is required")
		}
		if content == "" {
			return fmt.Errorf("content is required")
		}
		if entryType == "" {
			entryType = "decision"
		}

		// Open store
		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		// Create brain instance
		b := brain.NewBrain(s.DB())

		// Create entry
		entry := &brain.KnowledgeEntry{
			Type:     entryType,
			Title:    title,
			Content:  content,
			Tags:     tags,
			FileRefs: fileRefs,
			Status:   "temp",
		}

		// Save entry
		id, err := b.Save(entry, !yes)
		if err != nil {
			return fmt.Errorf("error saving entry: %w", err)
		}

		fmt.Printf("Knowledge entry saved with ID: %d\n", id)
		fmt.Printf("Type: %s\n", entry.Type)
		fmt.Printf("Title: %s\n", entry.Title)
		fmt.Printf("Status: %s\n", entry.Status)

		if !yes {
			fmt.Println("\nEntry saved as 'temp'. Use 'brain promote' to promote it.")
		}

		return nil
	},
}

var brainSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Busca en la memoria del proyecto",
	Long:  `Busca entradas de conocimiento por texto.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		// Open store
		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		// Create brain instance
		b := brain.NewBrain(s.DB())

		// Search
		entries, err := b.Search(query)
		if err != nil {
			return fmt.Errorf("error searching: %w", err)
		}

		if len(entries) == 0 {
			fmt.Println("No knowledge entries found")
			return nil
		}

		fmt.Printf("Found %d knowledge entries:\n\n", len(entries))
		for _, entry := range entries {
			fmt.Printf("ID: %d\n", entry.ID)
			fmt.Printf("Type: %s\n", entry.Type)
			fmt.Printf("Title: %s\n", entry.Title)
			fmt.Printf("Status: %s\n", entry.Status)
			fmt.Printf("Content: %s\n\n", truncateString(entry.Content, 100))
		}

		return nil
	},
}

var brainReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Revisa entradas temporales",
	Long:  `Lista todas las entradas de conocimiento en estado temporal.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Open store
		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		// Create brain instance
		b := brain.NewBrain(s.DB())

		// Review
		entries, err := b.Review()
		if err != nil {
			return fmt.Errorf("error reviewing: %w", err)
		}

		if len(entries) == 0 {
			fmt.Println("No temporary knowledge entries found")
			return nil
		}

		fmt.Printf("Found %d temporary knowledge entries:\n\n", len(entries))
		for _, entry := range entries {
			fmt.Printf("ID: %d\n", entry.ID)
			fmt.Printf("Type: %s\n", entry.Type)
			fmt.Printf("Title: %s\n", entry.Title)
			fmt.Printf("Content: %s\n\n", truncateString(entry.Content, 100))
		}

		return nil
	},
}

var brainPromoteCmd = &cobra.Command{
	Use:   "promote [id]",
	Short: "Promueve una entrada a estado promovido",
	Long:  `Cambia el estado de una entrada de conocimiento de 'temp' a 'promoted'.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Parse ID
		var id int64
		if _, err := fmt.Sscanf(args[0], "%d", &id); err != nil {
			return fmt.Errorf("invalid ID: %s", args[0])
		}

		// Open store
		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		// Create brain instance
		b := brain.NewBrain(s.DB())

		// Promote
		if err := b.Promote(id); err != nil {
			return fmt.Errorf("error promoting entry: %w", err)
		}

		fmt.Printf("Entry %d promoted successfully\n", id)
		return nil
	},
}

var brainExportCmd = &cobra.Command{
	Use:   "export [file]",
	Short: "Exporta la memoria del proyecto a Markdown",
	Long:  `Exporta todas las entradas de conocimiento a un archivo Markdown.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		// Open store
		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		// Create brain instance
		b := brain.NewBrain(s.DB())

		// Export
		if err := b.ExportToMarkdown(filePath); err != nil {
			return fmt.Errorf("error exporting: %w", err)
		}

		fmt.Printf("Knowledge exported to %s\n", filePath)
		return nil
	},
}

var brainImportCmd = &cobra.Command{
	Use:   "import [file]",
	Short: "Importa memoria del proyecto desde Markdown",
	Long:  `Importa entradas de conocimiento desde un archivo Markdown.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		// Open store
		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		// Create brain instance
		b := brain.NewBrain(s.DB())

		// Import
		if err := b.ImportFromMarkdown(filePath); err != nil {
			return fmt.Errorf("error importing: %w", err)
		}

		fmt.Printf("Knowledge imported from %s\n", filePath)
		return nil
	},
}

var brainSeedFromSemanticCmd = &cobra.Command{
	Use:   "seed-from-semantic",
	Short: "Siembra el brain con convenciones detectadas en file_semantics",
	Long: `Lee las clasificaciones de file_semantics (que produce 'androideai
semantic index'), las agrupa por tipo de archivo (ViewModel, UseCase,
Repository, Composable, ...) y guarda una entrada "convention" en el
brain por cada rol.

Por defecto es idempotente: si una entrada con el mismo título ya
existe, se la salta. Con --force borra las entradas existentes con
los títulos "<Role> convention" antes de re-sembrar, lo que permite
regenerar el brain después de clasificar archivos nuevos o mejorar
los prompts.

Útil cuando:
  - Querés re-sembrar el brain sin re-correr 'androideai init' entero.
  - Clasificaste archivos nuevos con 'androideai semantic index' y
    querés que el brain los vea.
  - Cambiaste el aggregator o el formato de las entradas.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")

		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		if force {
			deleted, err := deleteConventionEntries(s.DB())
			if err != nil {
				return fmt.Errorf("error borrando convenciones existentes: %w", err)
			}
			if deleted > 0 {
				fmt.Printf("  • Borradas %d entrada(s) existente(s) de tipo 'convention'.\n", deleted)
			}
		}

		if err := seedBrainFromSemantic(); err != nil {
			return fmt.Errorf("error sembrando brain: %w", err)
		}
		return nil
	},
}

// deleteConventionEntries borra todas las entradas del brain con
// type='convention'. Devuelve la cantidad borrada. Se usa por
// `brain seed-from-semantic --force` para permitir regenerar.
func deleteConventionEntries(db *sql.DB) (int, error) {
	res, err := db.Exec(`DELETE FROM knowledge_entries WHERE type = 'convention'`)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

var brainListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista todas las entradas de conocimiento",
	Long:  `Lista todas las entradas de conocimiento en la memoria del proyecto.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Open store
		dbPath := filepath.Join(".androideai", "core.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found, run 'androideai init' first")
		}

		s, err := store.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("error opening database: %w", err)
		}
		defer s.Close()

		// Create brain instance
		b := brain.NewBrain(s.DB())

		// List
		entries, err := b.List()
		if err != nil {
			return fmt.Errorf("error listing: %w", err)
		}

		if len(entries) == 0 {
			fmt.Println("No knowledge entries found")
			return nil
		}

		fmt.Printf("Found %d knowledge entries:\n\n", len(entries))
		for _, entry := range entries {
			fmt.Printf("ID: %d\n", entry.ID)
			fmt.Printf("Type: %s\n", entry.Type)
			fmt.Printf("Title: %s\n", entry.Title)
			fmt.Printf("Status: %s\n", entry.Status)
			fmt.Printf("Content: %s\n\n", truncateString(entry.Content, 100))
		}

		return nil
	},
}

func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength] + "..."
}

func init() {
	// Save command flags
	brainSaveCmd.Flags().StringP("type", "t", "decision", "Type of knowledge entry")
	brainSaveCmd.Flags().StringP("title", "i", "", "Title of the knowledge entry (required)")
	brainSaveCmd.Flags().StringP("content", "c", "", "Content of the knowledge entry (required)")
	brainSaveCmd.Flags().StringP("tags", "g", "", "Tags for the entry")
	brainSaveCmd.Flags().StringP("file-refs", "f", "", "File references")
	brainSaveCmd.Flags().BoolP("yes", "y", false, "Skip confirmation")

	// Seed-from-semantic flags
	brainSeedFromSemanticCmd.Flags().BoolP("force", "f", false,
		"Borra las entradas existentes de tipo 'convention' antes de re-sembrar")

	// Add commands
	brainCmd.AddCommand(brainSaveCmd)
	brainCmd.AddCommand(brainSearchCmd)
	brainCmd.AddCommand(brainReviewCmd)
	brainCmd.AddCommand(brainPromoteCmd)
	brainCmd.AddCommand(brainExportCmd)
	brainCmd.AddCommand(brainImportCmd)
	brainCmd.AddCommand(brainListCmd)
	brainCmd.AddCommand(brainSeedFromSemanticCmd)
}

// Helper function to read input from terminal
func readInput(prompt string) (string, error) {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("no input received")
}
