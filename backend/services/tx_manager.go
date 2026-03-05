package services

import (
	"context"

	"gorm.io/gorm"
)

// TxManager 抽象事务执行器，屏蔽具体 ORM 事务细节。
type TxManager interface {
	// WithTransaction 在单个数据库事务中执行回调，回调返回错误时应回滚事务。
	WithTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error
}
