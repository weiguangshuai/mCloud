package services

import "mcloud/repositories"

type Container struct {
	Auth       AuthService
	User       UserService
	Folder     FolderService
	File       FileService
	RecycleBin RecycleBinService
	Cleanup    CleanupService
}

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
