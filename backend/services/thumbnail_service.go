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

// IsImageFile 根据扩展名快速判断是否走图片处理流程。
func IsImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return imageExtensions[ext]
}

// GenerateThumbnail 按配置生成缩略图；会自动创建目标目录。
func GenerateThumbnail(srcPath, dstPath string) error {
	cfg := config.AppConfig

	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return fmt.Errorf("创建缩略图目录失败: %w", err)
	}

	img, err := imaging.Open(srcPath)
	if err != nil {
		return fmt.Errorf("打开图片失败: %w", err)
	}

	// 使用 Fit 保持原图比例，避免缩略图拉伸变形。
	thumb := imaging.Fit(img, cfg.Thumbnail.Width, cfg.Thumbnail.Height, imaging.Lanczos)
	return imaging.Save(thumb, dstPath, imaging.JPEGQuality(cfg.Thumbnail.Quality))
}

// GetImageDimensions 读取图片宽高，用于记录图片元数据。
func GetImageDimensions(filePath string) (int, int, error) {
	img, err := imaging.Open(filePath)
	if err != nil {
		return 0, 0, err
	}
	bounds := img.Bounds()
	return bounds.Dx(), bounds.Dy(), nil
}
