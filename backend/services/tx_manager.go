package services

import (
	"context"

	"gorm.io/gorm"
)

type TxManager interface {
	WithTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error
}
