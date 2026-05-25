package utils

import (
	"regexp"
	"testing"
)

func TestGenerateDynamicFlag_DefaultTemplate_UsesUUID32(t *testing.T) {
	re := regexp.MustCompile(`^flag\{[0-9a-f]{32}\}$`)
	for i := 0; i < 5; i++ {
		got := GenerateDynamicFlag("")
		if !re.MatchString(got) {
			t.Fatalf("expected default flag format flag{<uuid32>}, got=%q", got)
		}
	}
}

func TestGenerateDynamicFlag_CustomTemplate_UsesUUID32(t *testing.T) {
	re := regexp.MustCompile(`^EXECTF\{[0-9a-f]{32}\}$`)
	got := GenerateDynamicFlag("EXECTF{%s}")
	if !re.MatchString(got) {
		t.Fatalf("expected custom template with uuid32 payload, got=%q", got)
	}
}

func TestGenerateDynamicFlag_WithoutPlaceholder_AutoAppendsPayloadPlaceholder(t *testing.T) {
	re := regexp.MustCompile(`^EXECTF\{[0-9a-f]{32}\}$`)
	got := GenerateDynamicFlag("EXECTF{}")
	if !re.MatchString(got) {
		t.Fatalf("expected template to be normalized as EXECTF{%%s}, got=%q", got)
	}
}
