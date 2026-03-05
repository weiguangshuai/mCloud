package services

import "mcloud/repositories"

// Container 聚合所有服务实例，供 handler 层统一注入使用。
type Container struct {
	// Auth 负责登录注册与个人信息读取。
	Auth AuthService
	// User 负责用户配额等用户侧基础能力。
	User UserService
	// Folder 负责目录管理。
	Folder FolderService
	// File 负责文件上传下载与元数据管理。
	File FileService
	// RecycleBin 负责回收站查询、恢复与彻底删除。
	RecycleBin RecycleBinService
	// Cleanup 负责后台清理任务。
	Cleanup CleanupService
}

// NewContainer 组装服务实例并注册全局清理任务入口。
func NewContainer(repos repositories.Container) *Container {
	container := &Container{
		Auth:       NewAuthService(repos.TxManager, repos.Users, repos.Folders),
		User:       NewUserService(repos.Users),
		Folder:     NewFolderService(repos.TxManager, repos.Folders, repos.Files, repos.RecycleBin),
		File:       NewFileService(repos.TxManager, repos.Users, repos.Folders, repos.Files, repos.FileObjects, repos.UploadTasks, repos.RecycleBin, repos.UploadProgress),
		RecycleBin: NewRecycleBinService(repos.TxManager, repos.Users, repos.Folders, repos.Files, repos.FileObjects, repos.RecycleBin),
		Cleanup:    NewCleanupService(repos.TxManager, repos.Users, repos.Folders, repos.Files, repos.FileObjects, repos.UploadTasks, repos.RecycleBin),
	}
	SetCleanupService(container.Cleanup)
	return container
}
