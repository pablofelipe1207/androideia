package skills

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Skill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Triggers    []string `json:"triggers"`
	Path        string   `json:"path"`
	Source      string   `json:"source"` // "embedded", "global", "project"
	Content     string   `json:"content,omitempty"`
}

type SkillLoader struct {
	projectDir string
	globalDir  string
	skills     map[string]*Skill
}

func NewSkillLoader(projectDir string) *SkillLoader {
	homeDir, _ := os.UserHomeDir()
	globalDir := filepath.Join(homeDir, ".androideai", "skills")

	return &SkillLoader{
		projectDir: filepath.Join(projectDir, ".androideai", "skills"),
		globalDir:  globalDir,
		skills:     make(map[string]*Skill),
	}
}

func (l *SkillLoader) LoadAll() error {
	// Load in order: embedded < global < project (project wins)
	if err := l.loadEmbedded(); err != nil {
		return fmt.Errorf("error loading embedded skills: %w", err)
	}

	if err := l.loadFromDir(l.globalDir, "global"); err != nil {
		// Ignore if directory doesn't exist
		fmt.Printf("Note: Global skills directory not found at %s\n", l.globalDir)
	}

	if err := l.loadFromDir(l.projectDir, "project"); err != nil {
		// Ignore if directory doesn't exist
		fmt.Printf("Note: Project skills directory not found at %s\n", l.projectDir)
	}

	return nil
}

func (l *SkillLoader) loadEmbedded() error {
	// Note: Embedded skills are loaded from the skills/ directory at the project root
	// For now, we'll skip embedded skills and focus on project/global skills
	// In a real implementation, you would use embed.FS here
	return nil
}

func (l *SkillLoader) loadFromDir(dir, source string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(path, "SKILL.md") {
			skill, err := l.parseSkillFileFromOS(path)
			if err != nil {
				fmt.Printf("Warning: Error parsing skill %s: %v\n", path, err)
				return nil
			}
			skill.Source = source
			skill.Path = path
			l.skills[skill.Name] = skill
		}

		return nil
	})
}

func (l *SkillLoader) parseSkillFile(f fs.FS, path string) (*Skill, error) {
	data, err := fs.ReadFile(f, path)
	if err != nil {
		return nil, err
	}

	return parseSkillContent(string(data))
}

func (l *SkillLoader) parseSkillFileFromOS(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return parseSkillContent(string(data))
}

func parseSkillContent(content string) (*Skill, error) {
	skill := &Skill{}

	// Check for frontmatter
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("no frontmatter found")
	}

	// Find end of frontmatter
	endIdx := strings.Index(content[3:], "---")
	if endIdx == -1 {
		return nil, fmt.Errorf("unclosed frontmatter")
	}

	frontmatter := content[3 : endIdx+3]
	body := content[endIdx+6:]

	// Parse frontmatter (simple YAML parsing)
	lines := strings.Split(frontmatter, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		value = strings.Trim(value, "\"'")

		switch key {
		case "name":
			skill.Name = value
		case "description":
			skill.Description = value
		case "triggers":
			// Parse array
			value = strings.Trim(value, "[]")
			// Split by comma and clean up each trigger
			skill.Triggers = strings.Split(value, ",")
			for i, t := range skill.Triggers {
				// Remove quotes and whitespace
				t = strings.TrimSpace(t)
				t = strings.Trim(t, "\"'")
				skill.Triggers[i] = t
			}
		}
	}

	// Trim leading newline from body
	body = strings.TrimPrefix(body, "\n")
	skill.Content = body
	return skill, nil
}

func (l *SkillLoader) GetSkill(name string) (*Skill, error) {
	if skill, ok := l.skills[name]; ok {
		return skill, nil
	}
	return nil, fmt.Errorf("skill '%s' not found", name)
}

func (l *SkillLoader) ListSkills() []*Skill {
	var skills []*Skill
	for _, skill := range l.skills {
		skills = append(skills, skill)
	}

	// Sort by name
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})

	return skills
}

func (l *SkillLoader) AddSkill(sourcePath string) error {
	// Check if source exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("source path does not exist: %s", sourcePath)
	}

	// Read the skill file
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("error reading skill file: %w", err)
	}

	// Parse skill
	skill, err := parseSkillContent(string(data))
	if err != nil {
		return fmt.Errorf("error parsing skill file: %w", err)
	}

	// Create project skills directory if it doesn't exist
	if err := os.MkdirAll(l.projectDir, 0755); err != nil {
		return fmt.Errorf("error creating skills directory: %w", err)
	}

	// Copy skill to project directory
	skillDir := filepath.Join(l.projectDir, skill.Name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("error creating skill directory: %w", err)
	}

	// Copy SKILL.md
	destPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return fmt.Errorf("error copying skill file: %w", err)
	}

	// Reload skills
	l.skills = make(map[string]*Skill)
	return l.LoadAll()
}

func (l *SkillLoader) GetSkillPaths() []string {
	return []string{
		l.projectDir,
		l.globalDir,
		"embedded",
	}
}

func (l *SkillLoader) GetSkillContent(name string) (string, error) {
	skill, err := l.GetSkill(name)
	if err != nil {
		return "", err
	}
	return skill.Content, nil
}

func (l *SkillLoader) FindSkillsByTrigger(trigger string) []*Skill {
	var matches []*Skill
	trigger = strings.ToLower(trigger)

	for _, skill := range l.skills {
		for _, t := range skill.Triggers {
			if strings.Contains(strings.ToLower(t), trigger) {
				matches = append(matches, skill)
				break
			}
		}
	}

	return matches
}

func (l *SkillLoader) ExportSkillsJSON() (string, error) {
	skills := l.ListSkills()
	data, err := json.MarshalIndent(skills, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (l *SkillLoader) ImportFromOpencode() (int, error) {
	// Get opencode skills directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return 0, fmt.Errorf("error getting home directory: %w", err)
	}

	opencodeSkillsDir := filepath.Join(homeDir, ".config", "opencode", "skills")
	if _, err := os.Stat(opencodeSkillsDir); os.IsNotExist(err) {
		return 0, fmt.Errorf("opencode skills directory not found at %s", opencodeSkillsDir)
	}

	// Create project skills directory if it doesn't exist
	if err := os.MkdirAll(l.projectDir, 0755); err != nil {
		return 0, fmt.Errorf("error creating project skills directory: %w", err)
	}

	count := 0

	// Walk through opencode skills directory
	err = filepath.Walk(opencodeSkillsDir, func(path string, info os.FileInfo, err error) error {
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
			destDir := filepath.Join(l.projectDir, skillDir)
			if err := os.MkdirAll(destDir, 0755); err != nil {
				return fmt.Errorf("error creating skill directory: %w", err)
			}

			// Copy SKILL.md
			destPath := filepath.Join(destDir, "SKILL.md")
			if err := copyFile(path, destPath); err != nil {
				return fmt.Errorf("error copying skill file: %w", err)
			}

			// Copy references directory if it exists
			refsDir := filepath.Join(filepath.Dir(path), "references")
			if _, err := os.Stat(refsDir); err == nil {
				destRefsDir := filepath.Join(destDir, "references")
				if err := copyDir(refsDir, destRefsDir); err != nil {
					fmt.Printf("Warning: Error copying references directory: %v\n", err)
				}
			}

			count++
			fmt.Printf("Imported skill: %s\n", skillDir)
		}

		return nil
	})

	if err != nil {
		return count, fmt.Errorf("error importing skills: %w", err)
	}

	// Reload skills
	l.skills = make(map[string]*Skill)
	if err := l.LoadAll(); err != nil {
		return count, fmt.Errorf("error reloading skills: %w", err)
	}

	return count, nil
}

func copyFile(src, dst string) error {
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

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Create destination path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		return copyFile(path, destPath)
	})
}

func (l *SkillLoader) ImportFromURL(url string) error {
	// This is a placeholder for importing skills from a URL
	// In a real implementation, you would download and extract the skill
	return fmt.Errorf("URL import not implemented yet")
}

func (l *SkillLoader) ImportFromAndroidSkills() (int, error) {
	// Clone the Android skills repository to a temp directory
	tmpDir, err := os.MkdirTemp("", "android-skills-*")
	if err != nil {
		return 0, fmt.Errorf("error creating temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Clone the repository
	fmt.Println("Cloning Android skills repository...")
	cmd := exec.Command("git", "clone", "--depth", "1", "https://github.com/android/skills.git", tmpDir)
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("error cloning repository: %w", err)
	}

	// Create project skills directory if it doesn't exist
	if err := os.MkdirAll(l.projectDir, 0755); err != nil {
		return 0, fmt.Errorf("error creating project skills directory: %w", err)
	}

	count := 0

	// Walk through the cloned repository
	err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(path, "SKILL.md") {
			// Get the relative path from tmpDir
			relPath, err := filepath.Rel(tmpDir, path)
			if err != nil {
				return nil
			}

			// Create a skill name from the path (e.g., jetpack-compose/adaptive -> jetpack-compose-adaptive)
			skillName := strings.ReplaceAll(filepath.Dir(relPath), "/", "-")
			skillName = strings.TrimPrefix(skillName, "-")
			skillName = strings.TrimSuffix(skillName, "-")
			
			// Skip the root SKILL.md if it exists
			if skillName == "" {
				skillName = "android-skills"
			}

			// Create skill directory
			destDir := filepath.Join(l.projectDir, skillName)
			if err := os.MkdirAll(destDir, 0755); err != nil {
				return fmt.Errorf("error creating skill directory: %w", err)
			}

			// Copy SKILL.md
			destPath := filepath.Join(destDir, "SKILL.md")
			if err := copyFile(path, destPath); err != nil {
				return fmt.Errorf("error copying skill file: %w", err)
			}

			// Copy references directory if it exists
			refsDir := filepath.Join(filepath.Dir(path), "references")
			if _, err := os.Stat(refsDir); err == nil {
				destRefsDir := filepath.Join(destDir, "references")
				if err := copyDir(refsDir, destRefsDir); err != nil {
					fmt.Printf("Warning: Error copying references directory: %v\n", err)
				}
			}

			count++
			fmt.Printf("Imported skill: %s\n", skillName)
		}

		return nil
	})

	if err != nil {
		return count, fmt.Errorf("error importing skills: %w", err)
	}

	// Reload skills
	l.skills = make(map[string]*Skill)
	if err := l.LoadAll(); err != nil {
		return count, fmt.Errorf("error reloading skills: %w", err)
	}

	return count, nil
}

func (l *SkillLoader) ImportAndroidSkillByName(skillName string) error {
	// Clone the Android skills repository to a temp directory
	tmpDir, err := os.MkdirTemp("", "android-skills-*")
	if err != nil {
		return fmt.Errorf("error creating temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Clone the repository
	fmt.Println("Cloning Android skills repository...")
	cmd := exec.Command("git", "clone", "--depth", "1", "https://github.com/android/skills.git", tmpDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error cloning repository: %w", err)
	}

	// Find the skill in the repository
	skillPath := ""
	err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(path, "SKILL.md") {
			// Check if this is the skill we're looking for
			relPath, _ := filepath.Rel(tmpDir, path)
			dirName := strings.ReplaceAll(filepath.Dir(relPath), "/", "-")
			dirName = strings.TrimPrefix(dirName, "-")
			dirName = strings.TrimSuffix(dirName, "-")
			
			if dirName == skillName || strings.Contains(dirName, skillName) {
				skillPath = path
				return filepath.SkipDir
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("error finding skill: %w", err)
	}

	if skillPath == "" {
		return fmt.Errorf("skill '%s' not found in Android skills repository", skillName)
	}

	// Create project skills directory if it doesn't exist
	if err := os.MkdirAll(l.projectDir, 0755); err != nil {
		return fmt.Errorf("error creating project skills directory: %w", err)
	}

	// Create skill directory
	destDir := filepath.Join(l.projectDir, skillName)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("error creating skill directory: %w", err)
	}

	// Copy SKILL.md
	destPath := filepath.Join(destDir, "SKILL.md")
	if err := copyFile(skillPath, destPath); err != nil {
		return fmt.Errorf("error copying skill file: %w", err)
	}

	// Copy references directory if it exists
	refsDir := filepath.Join(filepath.Dir(skillPath), "references")
	if _, err := os.Stat(refsDir); err == nil {
		destRefsDir := filepath.Join(destDir, "references")
		if err := copyDir(refsDir, destRefsDir); err != nil {
			fmt.Printf("Warning: Error copying references directory: %v\n", err)
		}
	}

	fmt.Printf("Imported skill: %s\n", skillName)

	// Reload skills
	l.skills = make(map[string]*Skill)
	return l.LoadAll()
}
