package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mcloud/config"

	"github.com/disintegration/imaging"
)

var imageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true,
	".gif": true, ".bmp": true, ".webp": true,
}

func IsImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return imageExtensions[ext]
}

func GenerateThumbnail(srcPath, dstPath string) error {
	cfg := config.AppConfig

	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return fmt.Errorf("创建缩略图目录失败: %w", err)
	}

	img, err := imaging.Open(srcPath)
	if err != nil {
		return fmt.Errorf("打开图片失败: %w", err)
	}

	thumb := imaging.Fit(img, cfg.Thumbnail.Width, cfg.Thumbnail.Height, imaging.Lanczos)
	return imaging.Save(thumb, dstPath, imaging.JPEGQuality(cfg.Thumbnail.Quality))
}

func GetImageDimensions(filePath string) (int, int, error) {
	img, err := imaging.Open(filePath)
	if err != nil {
		return 0, 0, err
	}
	bounds := img.Bounds()
	return bounds.Dx(), bounds.Dy(), nil
}
