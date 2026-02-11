package main

import (
	"fmt"
	"log"
	"os"

	"mcloud/config"
	"mcloud/database"
	"mcloud/handlers"
	"mcloud/middleware"
	"mcloud/models"
	"mcloud/services"

	"github.com/gin-gonic/gin"
)

func main() {
	log.Println("mCloud 私人网盘系统启动中...")

	// 加载配置
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化 MySQL
	if err := database.InitMySQL(&cfg.Database); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	// 自动迁移数据库表
	database.DB.AutoMigrate(
		&models.User{},
		&models.Folder{},
		&models.FileObject{},
		&models.File{},
		&models.UploadTask{},
		&models.RecycleBinItem{},
		&models.ThumbnailTask{},
	)
	log.Println("数据库表迁移完成")

	// 初始化 Redis
	if err := database.InitRedis(&cfg.Redis); err != nil {
		log.Fatalf("初始化 Redis 失败: %v", err)
	}

	// 创建存储目录
	os.MkdirAll(cfg.Storage.BasePath+"/files", 0755)
	os.MkdirAll(cfg.Storage.BasePath+"/thumbnails", 0755)
	os.MkdirAll(cfg.Storage.BasePath+"/temp", 0755)

	// 启动后台清理任务
	services.StartCleanupWorkers()
	log.Println("后台清理任务已启动")

	// 设置路由
	r := gin.Default()
	r.Use(middleware.CORSMiddleware())
	setupRoutes(r)

	// 启动服务器
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("服务器启动在 http://%s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}

func setupRoutes(r *gin.Engine) {
	api := r.Group("/api")

	// 健康检查
	api.GET("/health", handlers.HealthCheck)

	// 认证（无需登录）
	auth := api.Group("/auth")
	{
		auth.POST("/register", handlers.Register)
		auth.POST("/login", handlers.Login)
	}

	// 需要认证的路由
	protected := api.Group("")
	protected.Use(middleware.AuthMiddleware())
	{
		// 用户信息
		protected.GET("/auth/profile", handlers.GetProfile)
		protected.GET("/user/storage/quota", handlers.GetStorageQuota)

		// 文件夹管理
		protected.GET("/folders", handlers.ListFolders)
		protected.POST("/folders", handlers.CreateFolder)
		protected.PUT("/folders/:id", handlers.RenameFolder)
		protected.DELETE("/folders/:id", handlers.DeleteFolder)

		// 文件管理
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

		// 回收站
		protected.GET("/recycle-bin", handlers.ListRecycleBin)
		protected.POST("/recycle-bin/:id/restore", handlers.RestoreItem)
		protected.DELETE("/recycle-bin/:id", handlers.PermanentDelete)
		protected.POST("/recycle-bin/empty", handlers.EmptyRecycleBin)
	}
}
