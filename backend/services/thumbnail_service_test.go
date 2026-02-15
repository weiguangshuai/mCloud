package services

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"mcloud/config"
)

func TestIsImageFile(t *testing.T) {
	if !IsImageFile("avatar.PNG") {
		t.Fatalf("expected PNG extension to be recognized")
	}
	if IsImageFile("doc.txt") {
		t.Fatalf("expected TXT extension to be rejected")
	}
}

func TestGenerateThumbnailAndReadDimensions(t *testing.T) {
	baseDir := t.TempDir()
	srcPath := filepath.Join(baseDir, "src.jpg")
	dstPath := filepath.Join(baseDir, "thumbs", "dst.jpg")

	src := image.NewRGBA(image.Rect(0, 0, 200, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 200; x++ {
			src.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}

	srcFile, err := os.Create(srcPath)
	if err != nil {
		t.Fatalf("failed to create src image: %v", err)
	}
	if err := jpeg.Encode(srcFile, src, &jpeg.Options{Quality: 95}); err != nil {
		_ = srcFile.Close()
		t.Fatalf("failed to write src image: %v", err)
	}
	if err := srcFile.Close(); err != nil {
		t.Fatalf("failed to close src image: %v", err)
	}

	config.AppConfig = &config.Config{
		Thumbnail: config.ThumbnailConfig{
			Width:   64,
			Height:  64,
			Quality: 80,
		},
	}

	if err := GenerateThumbnail(srcPath, dstPath); err != nil {
		t.Fatalf("GenerateThumbnail failed: %v", err)
	}

	width, height, err := GetImageDimensions(dstPath)
	if err != nil {
		t.Fatalf("GetImageDimensions failed: %v", err)
	}
	if width <= 0 || height <= 0 {
		t.Fatalf("expected positive dimensions, got %dx%d", width, height)
	}
	if width > 64 || height > 64 {
		t.Fatalf("thumbnail should be bounded by 64x64, got %dx%d", width, height)
	}
}
