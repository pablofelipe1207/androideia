package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mobiai/androideai-core/internal/skills"
	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Gestiona las skills del agente",
	Long:  `Lista, añade y gestiona las skills disponibles para el agente.`,
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista todas las skills disponibles",
	Long:  `Lista todas las skills disponibles en el sistema (embebidas, globales y del proyecto).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get current directory
		projectDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("error getting current directory: %w", err)
		}

		// Create skill loader
		loader := skills.NewSkillLoader(projectDir)
		if err := loader.LoadAll(); err != nil {
			return fmt.Errorf("error loading skills: %w", err)
		}

		// List skills
		skillList := loader.ListSkills()
		if len(skillList) == 0 {
			fmt.Println("No skills found")
			return nil
		}

		fmt.Printf("Available skills (%d):\n\n", len(skillList))
		for _, skill := range skillList {
			fmt.Printf("📦 %s\n", skill.Name)
			fmt.Printf("   Description: %s\n", skill.Description)
			fmt.Printf("   Source: %s\n", skill.Source)
			if len(skill.Triggers) > 0 {
				fmt.Printf("   Triggers: %v\n", skill.Triggers)
			}
			fmt.Println()
		}

		return nil
	},
}

var skillsAddCmd = &cobra.Command{
	Use:   "add [path]",
	Short: "Añade una nueva skill al proyecto",
	Long:  `Copia una skill desde una ruta especificada al directorio de skills del proyecto.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sourcePath := args[0]

		// Get current directory
		projectDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("error getting current directory: %w", err)
		}

		// Create skill loader
		loader := skills.NewSkillLoader(projectDir)

		// Add skill
		if err := loader.AddSkill(sourcePath); err != nil {
			return fmt.Errorf("error adding skill: %w", err)
		}

		fmt.Printf("Skill added successfully from: %s\n", sourcePath)
		fmt.Println("Run 'skills list' to see all available skills.")

		return nil
	},
}

var skillsPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Muestra las rutas de skills",
	Long:  `Muestra las rutas donde se buscan skills (proyecto, global, embebidas).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get current directory
		projectDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("error getting current directory: %w", err)
		}

		// Create skill loader
		loader := skills.NewSkillLoader(projectDir)

		// Get paths
		paths := loader.GetSkillPaths()

		fmt.Println("Skill paths (in order of precedence):")
		fmt.Println("1. Project:  ", paths[0])
		fmt.Println("2. Global:   ", paths[1])
		fmt.Println("3. Embedded: ", paths[2])

		return nil
	},
}

var skillsShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Muestra el contenido de una skill",
	Long:  `Muestra el contenido SKILL.md de una skill específica.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		skillName := args[0]

		// Get current directory
		projectDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("error getting current directory: %w", err)
		}

		// Create skill loader
		loader := skills.NewSkillLoader(projectDir)
		if err := loader.LoadAll(); err != nil {
			return fmt.Errorf("error loading skills: %w", err)
		}

		// Get skill
		skill, err := loader.GetSkill(skillName)
		if err != nil {
			return fmt.Errorf("error getting skill: %w", err)
		}

		fmt.Printf("Skill: %s\n", skill.Name)
		fmt.Printf("Source: %s\n", skill.Source)
		fmt.Printf("Path: %s\n\n", skill.Path)
		fmt.Println("---")
		fmt.Println(skill.Content)

		return nil
	},
}

var skillsSearchCmd = &cobra.Command{
	Use:   "search [trigger]",
	Short: "Busca skills por trigger",
	Long:  `Busca skills que coincidan con un trigger específico.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		trigger := args[0]

		// Get current directory
		projectDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("error getting current directory: %w", err)
		}

		// Create skill loader
		loader := skills.NewSkillLoader(projectDir)
		if err := loader.LoadAll(); err != nil {
			return fmt.Errorf("error loading skills: %w", err)
		}

		// Search by trigger
		matches := loader.FindSkillsByTrigger(trigger)
		if len(matches) == 0 {
			fmt.Printf("No skills found with trigger: %s\n", trigger)
			return nil
		}

		fmt.Printf("Skills matching trigger '%s':\n\n", trigger)
		for _, skill := range matches {
			fmt.Printf("📦 %s\n", skill.Name)
			fmt.Printf("   Description: %s\n", skill.Description)
			fmt.Printf("   Triggers: %v\n", skill.Triggers)
			fmt.Println()
		}

		return nil
	},
}

var skillsImportCmd = &cobra.Command{
	Use:   "import-opencode",
	Short: "Importa skills desde opencode",
	Long:  `Importa todas las skills disponibles en opencode (~/.config/opencode/skills/) al proyecto actual.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get current directory
		projectDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("error getting current directory: %w", err)
		}

		// Create skill loader
		loader := skills.NewSkillLoader(projectDir)

		// Import from opencode
		count, err := loader.ImportFromOpencode()
		if err != nil {
			return fmt.Errorf("error importing skills: %w", err)
		}

		fmt.Printf("Successfully imported %d skills from opencode\n", count)
		fmt.Println("Run 'skills list' to see all available skills.")

		return nil
	},
}

var skillsImportDirCmd = &cobra.Command{
	Use:   "import-dir [path]",
	Short: "Importa skills desde un directorio",
	Long:  `Importa skills desde un directorio específico al proyecto actual.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sourceDir := args[0]

		// Get current directory
		projectDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("error getting current directory: %w", err)
		}

		// Check if source directory exists
		if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
			return fmt.Errorf("source directory does not exist: %s", sourceDir)
		}

		// Copy skills from source directory to project
		count := 0
		err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			if strings.HasSuffix(path, "SKILL.md") {
				// Get skill directory name
				skillDir := filepath.Base(filepath.Dir(path))
				
				// Copy skill to project
				destDir := filepath.Join(projectDir, ".androideai", "skills", skillDir)
				if err := os.MkdirAll(destDir, 0755); err != nil {
					return fmt.Errorf("error creating skill directory: %w", err)
				}

				// Copy SKILL.md
				destPath := filepath.Join(destDir, "SKILL.md")
				if err := copySkillFile(path, destPath); err != nil {
					return fmt.Errorf("error copying skill file: %w", err)
				}

				count++
				fmt.Printf("Imported skill: %s\n", skillDir)
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("error importing skills: %w", err)
		}

		fmt.Printf("Successfully imported %d skills from %s\n", count, sourceDir)
		fmt.Println("Run 'skills list' to see all available skills.")

		return nil
	},
}

var skillsImportAndroidCmd = &cobra.Command{
	Use:   "import-android",
	Short: "Importa skills oficiales de Android",
	Long:  `Importa todas las skills oficiales de Android desde https://github.com/android/skills`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get current directory
		projectDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("error getting current directory: %w", err)
		}

		// Create skill loader
		loader := skills.NewSkillLoader(projectDir)

		// Import Android skills
		fmt.Println("Importing official Android skills...")
		count, err := loader.ImportFromAndroidSkills()
		if err != nil {
			return fmt.Errorf("error importing Android skills: %w", err)
		}

		fmt.Printf("Successfully imported %d Android skills\n", count)
		fmt.Println("Run 'skills list' to see all available skills.")

		return nil
	},
}

var skillsImportAndroidSkillCmd = &cobra.Command{
	Use:   "import-android-skill [name]",
	Short: "Importa una skill específica de Android",
	Long:  `Importa una skill específica del repositorio de Android skills.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		skillName := args[0]

		// Get current directory
		projectDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("error getting current directory: %w", err)
		}

		// Create skill loader
		loader := skills.NewSkillLoader(projectDir)

		// Import the specific Android skill
		fmt.Printf("Importing Android skill: %s...\n", skillName)
		err = loader.ImportAndroidSkillByName(skillName)
		if err != nil {
			return fmt.Errorf("error importing Android skill: %w", err)
		}

		fmt.Printf("Successfully imported Android skill: %s\n", skillName)
		fmt.Println("Run 'skills list' to see all available skills.")

		return nil
	},
}

func copySkillFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = destFile.ReadFrom(sourceFile)
	return err
}

func init() {
	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsAddCmd)
	skillsCmd.AddCommand(skillsPathCmd)
	skillsCmd.AddCommand(skillsShowCmd)
	skillsCmd.AddCommand(skillsSearchCmd)
	skillsCmd.AddCommand(skillsImportCmd)
	skillsCmd.AddCommand(skillsImportDirCmd)
	skillsCmd.AddCommand(skillsImportAndroidCmd)
	skillsCmd.AddCommand(skillsImportAndroidSkillCmd)
}
