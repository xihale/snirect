package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadConfigUpdatePresence ensures that when [update] section exists but
// rules_url is not set, the default RulesURL is still applied.
func TestLoadConfigUpdatePresence(t *testing.T) {
	tmpDir, err := os.UserConfigDir()
	if err != nil {
		t.Skip("cannot get user config dir")
	}
	testDir := filepath.Join(tmpDir, "snirect-test-loader")
	os.MkdirAll(testDir, 0o700)
	defer os.RemoveAll(testDir)

	cfgPath := filepath.Join(testDir, "config.toml")

	// Test case: [update] present with only auto_check_update set; expect other fields from defaults
	content := `[update]
auto_check_update = false
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// User set this to false, should be false
	if cfg.Update.AutoCheckUpdate != false {
		t.Errorf("AutoCheckUpdate = %v; want false", cfg.Update.AutoCheckUpdate)
	}

	// Default values should be restored for missing fields
	if cfg.Update.RulesURL == "" {
		t.Error("RulesURL is empty; expected default from PreparsedDefaultConfig")
	}
	if expected := PreparsedDefaultConfig.Update.RulesURL; cfg.Update.RulesURL != expected {
		t.Errorf("RulesURL = %q; want %q", cfg.Update.RulesURL, expected)
	}

	// Check a few other defaults
	if cfg.Update.AutoCheckRules != true {
		t.Errorf("AutoCheckRules = %v; want true", cfg.Update.AutoCheckRules)
	}
	if cfg.Update.AutoUpdate != true {
		t.Errorf("AutoUpdate = %v; want true", cfg.Update.AutoUpdate)
	}
	if cfg.Update.RulesCheckIntervalHours != 24 {
		t.Errorf("RulesCheckIntervalHours = %d; want 24", cfg.Update.RulesCheckIntervalHours)
	}
}
