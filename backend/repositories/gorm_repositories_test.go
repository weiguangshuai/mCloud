package repositories

import (
	"context"
	"errors"
	"testing"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	_ TxManager                 = (*GormTxManager)(nil)
	_ UserRepository            = (*GormUserRepository)(nil)
	_ FolderRepository          = (*GormFolderRepository)(nil)
	_ FileRepository            = (*GormFileRepository)(nil)
	_ FileObjectRepository      = (*GormFileObjectRepository)(nil)
	_ UploadTaskRepository      = (*GormUploadTaskRepository)(nil)
	_ RecycleBinRepository      = (*GormRecycleBinRepository)(nil)
	_ UploadProgressRepository  = (*RedisUploadProgressRepository)(nil)
)

func TestGormTxManager_WithTransaction_Success(t *testing.T) {
	db, _ := newDryRunMySQL(t)
	manager := NewGormTxManager(db)

	called := false
	err := manager.WithTransaction(context.Background(), func(tx *gorm.DB) error {
		called = true
		if tx == nil {
			t.Fatalf("transaction db should not be nil")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithTransaction failed: %v", err)
	}
	if !called {
		t.Fatalf("transaction callback should be called")
	}
}

func TestGormTxManager_WithTransaction_PropagatesError(t *testing.T) {
	db, _ := newDryRunMySQL(t)
	manager := NewGormTxManager(db)
	wantErr := errors.New("boom")

	err := manager.WithTransaction(context.Background(), func(_ *gorm.DB) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestGormRepositories_BuildContainer_WiresAllRepositories(t *testing.T) {
	db, _ := newDryRunMySQL(t)
	redisClient := redis.NewClient(&redis.Options{
		Addr:            "127.0.0.1:0",
		Protocol:        2,
		DisableIdentity: true,
	})
	t.Cleanup(func() { _ = redisClient.Close() })

	repos := NewGormRepositories(db, redisClient)
	container := repos.BuildContainer()

	if container.TxManager == nil {
		t.Fatalf("TxManager should not be nil")
	}
	if container.Users == nil {
		t.Fatalf("Users should not be nil")
	}
	if container.Folders == nil {
		t.Fatalf("Folders should not be nil")
	}
	if container.Files == nil {
		t.Fatalf("Files should not be nil")
	}
	if container.FileObjects == nil {
		t.Fatalf("FileObjects should not be nil")
	}
	if container.UploadTasks == nil {
		t.Fatalf("UploadTasks should not be nil")
	}
	if container.RecycleBin == nil {
		t.Fatalf("RecycleBin should not be nil")
	}
	if container.UploadProgress == nil {
		t.Fatalf("UploadProgress should not be nil")
	}
}

func TestUseTx_ReturnsTxWhenProvided(t *testing.T) {
	db, _ := newDryRunMySQL(t)
	tx := db.Session(&gorm.Session{NewDB: true})

	got := useTx(db, tx)
	if got != tx {
		t.Fatalf("expected tx to be returned when provided")
	}
}

func TestUseTx_FallsBackToDBWhenTxNil(t *testing.T) {
	db, _ := newDryRunMySQL(t)

	got := useTx(db, nil)
	if got != db {
		t.Fatalf("expected db to be returned when tx is nil")
	}
}
