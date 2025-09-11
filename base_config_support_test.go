package modular

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBaseConfigSupportEnableDisable(t *testing.T) {
	// ensure disabled path returns nil feeder
	BaseConfigSettings.Enabled = false
	if GetBaseConfigFeeder() != nil {
		t.Fatalf("expected nil feeder when disabled")
	}

	SetBaseConfig("configs", "dev")
	if !IsBaseConfigEnabled() {
		t.Fatalf("expected enabled after SetBaseConfig")
	}
	if GetBaseConfigFeeder() == nil {
		t.Fatalf("expected feeder when enabled")
	}
	if GetBaseConfigComplexFeeder() == nil {
		t.Fatalf("expected complex feeder when enabled")
	}
}

func TestDetectBaseConfigStructureNone(t *testing.T) {
	// run in temp dir without structure
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	dir := t.TempDir()
	os.Chdir(dir)
	BaseConfigSettings.Enabled = false
	if DetectBaseConfigStructure() {
		t.Fatalf("should not detect structure")
	}
}

// TestDetectEnvironmentDirectory ensures DetectBaseConfigStructure chooses the first environment when none specified.
func TestDetectEnvironmentDirectory(t *testing.T) {
	base := t.TempDir()
	// construct minimal base config structure
	if err := os.MkdirAll(filepath.Join(base, "config", "base"), 0o755); err != nil {
		t.Fatalf("mkdir base: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(base, "config", "environments", "staging"), 0o755); err != nil {
		t.Fatalf("mkdir env: %v", err)
	}
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(base); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	BaseConfigSettings.Enabled = false
	if !DetectBaseConfigStructure() {
		t.Fatalf("expected structure detection")
	}
	if BaseConfigSettings.Environment != "staging" {
		t.Fatalf("expected staging got %s", BaseConfigSettings.Environment)
	}
}

// TestDetectEnvironmentVariable ensures ENV overrides directory detection.
func TestDetectEnvironmentVariable(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "config", "base"), 0o755); err != nil {
		t.Fatalf("mkdir base: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(base, "config", "environments", "staging"), 0o755); err != nil {
		t.Fatalf("mkdir env: %v", err)
	}
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(base); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	old := os.Getenv("ENV")
	defer os.Setenv("ENV", old)
	os.Setenv("ENV", "production")
	BaseConfigSettings.Enabled = false
	if !DetectBaseConfigStructure() {
		t.Fatalf("expected structure detection")
	}
	if BaseConfigSettings.Environment != "production" {
		t.Fatalf("expected production got %s", BaseConfigSettings.Environment)
	}
}
