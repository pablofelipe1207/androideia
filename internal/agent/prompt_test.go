package agent

import (
	"strings"
	"testing"

	"github.com/pablofelipe1207/androideia/internal/project"
)

func TestBuildProjectContextBlock_NilReturnsEmpty(t *testing.T) {
	if got := BuildProjectContextBlock(nil); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestBuildProjectContextBlock_EmptyMetadataReturnsEmpty(t *testing.T) {
	if got := BuildProjectContextBlock(&project.Metadata{}); got != "" {
		t.Errorf("expected empty string for empty metadata, got %q", got)
	}
}

func TestBuildProjectContextBlock_FullMetadata(t *testing.T) {
	md := &project.Metadata{
		AppPath:            ".",
		ManifestPath:       "app/src/main/AndroidManifest.xml",
		ApplicationID:      "com.example.myapplication",
		ManifestActivities: []string{".MainActivity", ".feature.detail.DetailActivity"},
		LibsVersionsPath:   "gradle/libs.versions.toml",
		LibsVersions: map[string]string{
			"agp":         "8.2.0",
			"compose-bom": "2024.02.00",
		},
		LibsLibraries: map[string]string{
			"androidx-core-ktx": "androidx.core:core-ktx",
		},
	}
	block := BuildProjectContextBlock(md)

	for _, want := range []string{
		"## Project context",
		"com.example.myapplication",
		"app/src/main/AndroidManifest.xml",
		"MainActivity",
		"gradle/libs.versions.toml",
		"compose-bom",
		"androidx-core-ktx",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("block missing %q\nfull block:\n%s", want, block)
		}
	}
}

func TestSystemPrompt_MentionsPackageAndLibsRules(t *testing.T) {
	for _, want := range []string{
		"NEVER invent a package name",
		"AndroidManifest",
		"namespace",
		"libs.versions.toml",
		"REUSE it", // continuar con "by alias" en la línea siguiente
		"REUSE the alias",
		"Do NOT redeclare",
	} {
		if !strings.Contains(SystemPrompt, want) {
			t.Errorf("SystemPrompt missing rule %q", want)
		}
	}
}

func TestSetProjectMetadata_InjectsBlockIntoSystemPrompt(t *testing.T) {
	ag := NewAgent(nil, nil, nil)
	md := &project.Metadata{
		ApplicationID: "com.example.myapplication",
		ManifestActivities: []string{".MainActivity"},
		LibsVersionsPath: "gradle/libs.versions.toml",
		LibsVersions: map[string]string{"agp": "8.2.0"},
		LibsLibraries: map[string]string{"hilt-android": "com.google.dagger:hilt-android"},
	}
	ag.SetProjectMetadata(md)

	if !strings.Contains(ag.messages[0].Content, "## Project context") {
		t.Fatal("system prompt does not contain the project context block")
	}
	if !strings.Contains(ag.messages[0].Content, "com.example.myapplication") {
		t.Error("system prompt does not contain the applicationId")
	}
}

func TestSetProjectMetadata_Idempotent(t *testing.T) {
	ag := NewAgent(nil, nil, nil)
	md := &project.Metadata{ApplicationID: "com.example.app"}
	ag.SetProjectMetadata(md)
	firstLen := len(ag.messages[0].Content)
	ag.SetProjectMetadata(md)
	if len(ag.messages[0].Content) != firstLen {
		t.Errorf("expected idempotent SetProjectMetadata, lengths differ: %d vs %d",
			firstLen, len(ag.messages[0].Content))
	}
}

func TestSetProjectMetadata_NilIsNoop(t *testing.T) {
	ag := NewAgent(nil, nil, nil)
	original := ag.messages[0].Content
	ag.SetProjectMetadata(nil)
	if ag.messages[0].Content != original {
		t.Error("SetProjectMetadata(nil) should not modify messages")
	}
}
