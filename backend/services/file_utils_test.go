package services

import (
	"testing"

	"mcloud/config"
)

func TestSanitizeFilename(t *testing.T) {
	got := sanitizeFilename("../foo\\bar.txt")
	if got != "bar.txt" {
		t.Fatalf("expected bar.txt, got %s", got)
	}
}

func TestIsFileExtensionAllowed(t *testing.T) {
	config.AppConfig = &config.Config{Storage: config.StorageConfig{AllowedExtensions: []string{".jpg", ".png"}}}
	if !isFileExtensionAllowed("a.JPG") {
		t.Fatalf("expected JPG to be allowed")
	}
	if isFileExtensionAllowed("a.exe") {
		t.Fatalf("expected EXE to be blocked")
	}
}
