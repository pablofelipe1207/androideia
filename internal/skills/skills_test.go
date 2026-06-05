package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewSkillLoader(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "skills-loader-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create skill loader
	loader := NewSkillLoader(tmpDir)
	if loader == nil {
		t.Fatal("Skill loader is nil")
	}

	// Check that loader has correct paths
	if loader.projectDir != filepath.Join(tmpDir, ".androideai", "skills") {
		t.Errorf("Expected project dir '%s', got '%s'", filepath.Join(tmpDir, ".androideai", "skills"), loader.projectDir)
	}
}

func TestParseSkillContent(t *testing.T) {
	// Test parsing valid skill content
	content := `---
name: test-skill
description: A test skill for testing
triggers: ["test", "testing"]
---

# Test Skill

This is a test skill content.
`

	skill, err := parseSkillContent(content)
	if err != nil {
		t.Fatalf("Error parsing skill content: %v", err)
	}

	if skill.Name != "test-skill" {
		t.Errorf("Expected name 'test-skill', got '%s'", skill.Name)
	}

	if skill.Description != "A test skill for testing" {
		t.Errorf("Expected description 'A test skill for testing', got '%s'", skill.Description)
	}

	if len(skill.Triggers) != 2 {
		t.Errorf("Expected 2 triggers, got %d", len(skill.Triggers))
	}

	if skill.Triggers[0] != "test" {
		t.Errorf("Expected first trigger 'test', got '%s'", skill.Triggers[0])
	}

	if skill.Triggers[1] != "testing" {
		t.Errorf("Expected second trigger 'testing', got '%s'", skill.Triggers[1])
	}

	// Content should start after the frontmatter (may have leading newline)
	if !strings.Contains(skill.Content, "# Test Skill") {
		t.Errorf("Content does not contain expected header: %s", skill.Content)
	}
	if !strings.Contains(skill.Content, "This is a test skill content.") {
		t.Errorf("Content does not contain expected body: %s", skill.Content)
	}
}

func TestParseSkillContentNoFrontmatter(t *testing.T) {
	// Test parsing skill content without frontmatter
	content := `# Test Skill

This is a test skill content.
`

	_, err := parseSkillContent(content)
	if err == nil {
		t.Error("Expected error for skill without frontmatter")
	}
}

func TestLoadAll(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "skills-loadall-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create project skills directory
	projectDir := filepath.Join(tmpDir, ".androideai", "skills", "my-skill")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Error creating project skills directory: %v", err)
	}

	// Create a skill file
	skillContent := `---
name: my-skill
description: My custom skill
triggers: ["custom", "my"]
---

# My Skill

This is my custom skill.
`

	skillFile := filepath.Join(projectDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Error creating skill file: %v", err)
	}

	// Create skill loader
	loader := NewSkillLoader(tmpDir)

	// Load all skills
	if err := loader.LoadAll(); err != nil {
		t.Fatalf("Error loading skills: %v", err)
	}

	// Check that skills were loaded
	skills := loader.ListSkills()
	if len(skills) == 0 {
		t.Error("No skills loaded")
	}

	// Find our custom skill
	found := false
	for _, skill := range skills {
		if skill.Name == "my-skill" {
			found = true
			if skill.Source != "project" {
				t.Errorf("Expected source 'project', got '%s'", skill.Source)
			}
			break
		}
	}

	if !found {
		t.Error("Custom skill not found")
	}
}

func TestGetSkill(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "skills-getskill-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create project skills directory
	projectDir := filepath.Join(tmpDir, ".androideai", "skills", "test-skill")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Error creating project skills directory: %v", err)
	}

	// Create a skill file
	skillContent := `---
name: test-skill
description: A test skill
triggers: ["test"]
---

# Test Skill

This is a test skill.
`

	skillFile := filepath.Join(projectDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Error creating skill file: %v", err)
	}

	// Create skill loader
	loader := NewSkillLoader(tmpDir)

	// Load all skills
	if err := loader.LoadAll(); err != nil {
		t.Fatalf("Error loading skills: %v", err)
	}

	// Get skill
	skill, err := loader.GetSkill("test-skill")
	if err != nil {
		t.Fatalf("Error getting skill: %v", err)
	}

	if skill.Name != "test-skill" {
		t.Errorf("Expected name 'test-skill', got '%s'", skill.Name)
	}

	// Try to get non-existent skill
	_, err = loader.GetSkill("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent skill")
	}
}

func TestListSkills(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "skills-list-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create multiple skill directories
	skills := []string{"skill-a", "skill-b", "skill-c"}
	for _, skillName := range skills {
		skillDir := filepath.Join(tmpDir, ".androideai", "skills", skillName)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatalf("Error creating skill directory: %v", err)
		}

		skillContent := `---
name: ` + skillName + `
description: Skill ` + skillName + `
triggers: ["` + skillName + `"]
---

# Skill ` + skillName + `
`
		skillFile := filepath.Join(skillDir, "SKILL.md")
		if err := os.WriteFile(skillFile, []byte(skillContent), 0644); err != nil {
			t.Fatalf("Error creating skill file: %v", err)
		}
	}

	// Create skill loader
	loader := NewSkillLoader(tmpDir)

	// Load all skills
	if err := loader.LoadAll(); err != nil {
		t.Fatalf("Error loading skills: %v", err)
	}

	// List skills
	skillList := loader.ListSkills()
	if len(skillList) != 3 {
		t.Errorf("Expected 3 skills, got %d", len(skillList))
	}

	// Check that skills are sorted by name
	for i := 0; i < len(skillList)-1; i++ {
		if skillList[i].Name >= skillList[i+1].Name {
			t.Errorf("Skills not sorted: %s >= %s", skillList[i].Name, skillList[i+1].Name)
		}
	}
}

func TestFindSkillsByTrigger(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "skills-findtrigger-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create skill with triggers
	skillDir := filepath.Join(tmpDir, ".androideai", "skills", "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Error creating skill directory: %v", err)
	}

	skillContent := `---
name: test-skill
description: A test skill
triggers: ["create feature", "new screen", "add screen"]
---

# Test Skill
`
	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Error creating skill file: %v", err)
	}

	// Create skill loader
	loader := NewSkillLoader(tmpDir)

	// Load all skills
	if err := loader.LoadAll(); err != nil {
		t.Fatalf("Error loading skills: %v", err)
	}

	// Find by trigger
	matches := loader.FindSkillsByTrigger("create")
	if len(matches) != 1 {
		t.Errorf("Expected 1 match, got %d", len(matches))
	}

	// Find by partial trigger
	matches = loader.FindSkillsByTrigger("screen")
	if len(matches) != 1 {
		t.Errorf("Expected 1 match for 'screen', got %d", len(matches))
	}

	// Find by non-existent trigger
	matches = loader.FindSkillsByTrigger("nonexistent")
	if len(matches) != 0 {
		t.Errorf("Expected 0 matches for 'nonexistent', got %d", len(matches))
	}
}

func TestGetSkillPaths(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "skills-paths-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create skill loader
	loader := NewSkillLoader(tmpDir)

	// Get paths
	paths := loader.GetSkillPaths()
	if len(paths) != 3 {
		t.Errorf("Expected 3 paths, got %d", len(paths))
	}

	// Check project path
	expectedProjectPath := filepath.Join(tmpDir, ".androideai", "skills")
	if paths[0] != expectedProjectPath {
		t.Errorf("Expected project path '%s', got '%s'", expectedProjectPath, paths[0])
	}
}

func TestExportSkillsJSON(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "skills-export-test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create project skills directory
	skillDir := filepath.Join(tmpDir, ".androideai", "skills", "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Error creating skill directory: %v", err)
	}

	// Create a skill file
	skillContent := `---
name: test-skill
description: A test skill
triggers: ["test"]
---

# Test Skill
`
	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Error creating skill file: %v", err)
	}

	// Create skill loader
	loader := NewSkillLoader(tmpDir)

	// Load all skills
	if err := loader.LoadAll(); err != nil {
		t.Fatalf("Error loading skills: %v", err)
	}

	// Export to JSON
	jsonStr, err := loader.ExportSkillsJSON()
	if err != nil {
		t.Fatalf("Error exporting skills JSON: %v", err)
	}

	if len(jsonStr) == 0 {
		t.Error("Exported JSON is empty")
	}

	// Check that it contains the skill name
	if !containsString(jsonStr, "test-skill") {
		t.Error("Exported JSON does not contain skill name")
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
