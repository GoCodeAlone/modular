package modular

import (
    "os"
    "testing"
)

func TestBaseConfigSupportEnableDisable(t *testing.T) {
    // ensure disabled path returns nil feeder
    BaseConfigSettings.Enabled = false
    if GetBaseConfigFeeder() != nil { t.Fatalf("expected nil feeder when disabled") }

    SetBaseConfig("configs", "dev")
    if !IsBaseConfigEnabled() { t.Fatalf("expected enabled after SetBaseConfig") }
    if GetBaseConfigFeeder() == nil { t.Fatalf("expected feeder when enabled") }
    if GetBaseConfigComplexFeeder() == nil { t.Fatalf("expected complex feeder when enabled") }
}

func TestDetectBaseConfigStructureNone(t *testing.T) {
    // run in temp dir without structure
    wd, _ := os.Getwd()
    defer os.Chdir(wd)
    dir := t.TempDir()
    os.Chdir(dir)
    BaseConfigSettings.Enabled = false
    if DetectBaseConfigStructure() { t.Fatalf("should not detect structure") }
}
