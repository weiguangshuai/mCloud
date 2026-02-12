package repositories

import (
	"context"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type GormTxManager struct {
	db *gorm.DB
}

func NewGormTxManager(db *gorm.DB) *GormTxManager {
	return &GormTxManager{db: db}
}

func (m *GormTxManager) WithTransaction(_ context.Context, fn func(tx *gorm.DB) error) error {
	return m.db.Transaction(fn)
}

type GormRepositories struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewGormRepositories(db *gorm.DB, redisClient *redis.Client) *GormRepositories {
	return &GormRepositories{db: db, redis: redisClient}
}

func (r *GormRepositories) BuildContainer() Container {
	return Container{
		TxManager:      NewGormTxManager(r.db),
		Users:          NewGormUserRepository(r.db),
		Folders:        NewGormFolderRepository(r.db),
		Files:          NewGormFileRepository(r.db),
		FileObjects:    NewGormFileObjectRepository(r.db),
		UploadTasks:    NewGormUploadTaskRepository(r.db),
		RecycleBin:     NewGormRecycleBinRepository(r.db),
		UploadProgress: NewRedisUploadProgressRepository(r.redis),
	}
}

func useTx(db *gorm.DB, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return db
}
