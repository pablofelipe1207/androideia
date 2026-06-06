package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig_TimeoutIs300(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Timeout != 300 {
		t.Errorf("expected default timeout 300, got %d", cfg.Timeout)
	}
}

func TestEffectiveTimeout(t *testing.T) {
	cases := []struct {
		name    string
		timeout int
		want    int
	}{
		{"zero uses default", 0, 300},
		{"negative uses default", -1, 300},
		{"explicit 600", 600, 600},
		{"explicit 60", 60, 60},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{Timeout: tc.timeout}
			if got := cfg.EffectiveTimeout(); got != tc.want {
				t.Errorf("EffectiveTimeout() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestConfigFile_LoadsTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yml")
	yaml := `model: foo
timeout: 900
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := LoadConfigFromFile(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Timeout != 900 {
		t.Errorf("expected timeout 900, got %d", cfg.Timeout)
	}
	if cfg.EffectiveTimeout() != 900 {
		t.Errorf("expected effective timeout 900, got %d", cfg.EffectiveTimeout())
	}
}
