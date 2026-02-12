package services

import (
	"path/filepath"
	"strings"

	"mcloud/config"
)

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	replacer := strings.NewReplacer("..", "_", "/", "_", "\\", "_")
	return replacer.Replace(name)
}

func isFileExtensionAllowed(fileName string) bool {
	allowed := config.AppConfig.Storage.AllowedExtensions
	if len(allowed) == 0 {
		return true
	}

	fileExt := strings.ToLower(filepath.Ext(fileName))
	for _, ext := range allowed {
		normalized := strings.ToLower(strings.TrimSpace(ext))
		if normalized == "*" {
			return true
		}
		if normalized == "" {
			continue
		}
		if !strings.HasPrefix(normalized, ".") {
			normalized = "." + normalized
		}
		if normalized == fileExt {
			return true
		}
	}

	return false
}

func getMimeType(ext string) string {
	mimeTypes := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".bmp":  "image/bmp",
		".webp": "image/webp",
		".pdf":  "application/pdf",
		".txt":  "text/plain",
		".mp4":  "video/mp4",
		".mp3":  "audio/mpeg",
		".zip":  "application/zip",
		".doc":  "application/msword",
	}
	if mt, ok := mimeTypes[strings.ToLower(ext)]; ok {
		return mt
	}
	return "application/octet-stream"
}
