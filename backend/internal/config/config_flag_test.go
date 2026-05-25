package config

import "testing"

func TestLoadFlagConfig_DefaultTemplate(t *testing.T) {
	t.Setenv("FLAG_TEMPLATE", "")
	cfg, err := loadFlagConfig()
	if err != nil {
		t.Fatalf("loadFlagConfig returned error: %v", err)
	}
	if cfg.Template != "flag{%s}" {
		t.Fatalf("expected default template flag{%%s}, got=%q", cfg.Template)
	}
}

func TestLoadFlagConfig_AutoFixTemplateWithoutPlaceholder(t *testing.T) {
	t.Setenv("FLAG_TEMPLATE", "EXECTF{}")
	cfg, err := loadFlagConfig()
	if err != nil {
		t.Fatalf("loadFlagConfig returned error: %v", err)
	}
	if cfg.Template != "EXECTF{%s}" {
		t.Fatalf("expected normalized template EXECTF{%%s}, got=%q", cfg.Template)
	}
}
