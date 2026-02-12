package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"mcloud/config"
	"mcloud/database"
	"mcloud/handlers"
	"mcloud/middleware"
	"mcloud/models"
	"mcloud/repositories"
	"mcloud/services"

	"github.com/gin-gonic/gin"
)

func main() {
	log.Println("starting mCloud service")

	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	if err := database.InitMySQL(&cfg.Database); err != nil {
		log.Fatalf("init mysql failed: %v", err)
	}

	database.DB.AutoMigrate(
		&models.User{},
		&models.Folder{},
		&models.FileObject{},
		&models.File{},
		&models.UploadTask{},
		&models.RecycleBinItem{},
		&models.ThumbnailTask{},
	)
	log.Println("database migration completed")

	if err := database.InitRedis(&cfg.Redis); err != nil {
		log.Fatalf("init redis failed: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(cfg.Storage.BasePath, "files"), 0o755); err != nil {
		log.Fatalf("create files dir failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.Storage.BasePath, "thumbnails"), 0o755); err != nil {
		log.Fatalf("create thumbnails dir failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.Storage.BasePath, "temp"), 0o755); err != nil {
		log.Fatalf("create temp dir failed: %v", err)
	}

	repoContainer := repositories.NewGormRepositories(database.DB, database.RedisClient).BuildContainer()
	serviceContainer := services.NewContainer(repoContainer)
	handlers.SetServices(serviceContainer)

	services.StartCleanupWorkers()
	log.Println("cleanup workers started")

	r := gin.Default()
	r.Use(middleware.CORSMiddleware())
	setupRoutes(r)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("server listening on http://%s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server start failed: %v", err)
	}
}

func setupRoutes(r *gin.Engine) {
	api := r.Group("/api")

	api.GET("/health", handlers.HealthCheck)

	auth := api.Group("/auth")
	{
		auth.POST("/register", handlers.Register)
		auth.POST("/login", handlers.Login)
	}

	protected := api.Group("")
	protected.Use(middleware.AuthMiddleware())
	{
		protected.GET("/auth/profile", handlers.GetProfile)
		protected.GET("/user/storage/quota", handlers.GetStorageQuota)

		protected.GET("/folders", handlers.ListFolders)
		protected.POST("/folders", handlers.CreateFolder)
		protected.PUT("/folders/:id", handlers.RenameFolder)
		protected.DELETE("/folders/:id", handlers.DeleteFolder)

		protected.GET("/files", handlers.ListFiles)
		protected.POST("/files/upload", handlers.UploadFile)
		protected.POST("/files/upload/init", handlers.InitChunkedUpload)
		protected.POST("/files/upload/chunk", handlers.UploadChunk)
		protected.POST("/files/upload/complete", handlers.CompleteUpload)
		protected.GET("/files/upload/status/:upload_id", handlers.GetUploadStatus)
		protected.GET("/files/:id/download", handlers.DownloadFile)
		protected.HEAD("/files/:id/download", handlers.DownloadFileHead)
		protected.GET("/files/:id/preview", handlers.PreviewFile)
		protected.GET("/files/:id/thumbnail", handlers.GetThumbnail)
		protected.DELETE("/files/:id", handlers.DeleteFile)
		protected.PUT("/files/:id/rename", handlers.RenameFile)
		protected.PUT("/files/:id/move", handlers.MoveFile)
		protected.POST("/files/batch/delete", handlers.BatchDeleteFiles)
		protected.POST("/files/batch/move", handlers.BatchMoveFiles)
		protected.POST("/files/thumbnails/batch", handlers.BatchGetThumbnails)

		protected.GET("/recycle-bin", handlers.ListRecycleBin)
		protected.POST("/recycle-bin/:id/restore", handlers.RestoreItem)
		protected.DELETE("/recycle-bin/:id", handlers.PermanentDelete)
		protected.POST("/recycle-bin/empty", handlers.EmptyRecycleBin)
	}
}
