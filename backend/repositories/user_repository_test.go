package repositories

import (
	"context"
	"testing"

	"mcloud/models"

	"gorm.io/gorm"
)

func TestGormUserRepository_CountByUsername_BuildsExpectedSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormUserRepository(db)

	count, err := repo.CountByUsername(context.Background(), "alice")
	if err != nil {
		t.Fatalf("CountByUsername failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected dry-run count 0, got %d", count)
	}

	assertLastSQLContains(t, rec, "select count(*)", "from `users`", "where username = ?")
}

func TestGormUserRepository_Create_BuildsInsertSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormUserRepository(db)

	user := &models.User{
		Username: "alice",
		Password: "secret",
		Nickname: "Alice",
	}
	if err := repo.Create(context.Background(), nil, user); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	assertLastSQLContains(t, rec, "insert into `users`")
}

func TestGormUserRepository_GetByUsername_BuildsSelectSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormUserRepository(db)

	_, err := repo.GetByUsername(context.Background(), nil, "alice")
	if err != nil {
		t.Fatalf("GetByUsername failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `users`", "where username = ?")
}

func TestGormUserRepository_GetByID_BuildsSelectByPrimaryKeySQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormUserRepository(db)

	_, err := repo.GetByID(context.Background(), nil, 7)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `users`", "where `users`.`id` = ?")
}

func TestGormUserRepository_AddStorageUsed_DeltaZero_NoSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormUserRepository(db)

	if err := repo.AddStorageUsed(context.Background(), nil, 1, 0); err != nil {
		t.Fatalf("AddStorageUsed should return nil when delta is zero: %v", err)
	}

	assertNoSQLCaptured(t, rec)
}

func TestGormUserRepository_AddStorageUsed_Positive_BuildsUpdateSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormUserRepository(db)

	tx := db.Session(&gorm.Session{})
	if err := repo.AddStorageUsed(context.Background(), tx, 1, 1024); err != nil {
		t.Fatalf("AddStorageUsed failed: %v", err)
	}

	assertLastSQLContains(t, rec, "update `users`", "`storage_used`=storage_used + ?", "where id = ?")
}

func TestGormUserRepository_SubStorageUsed_NonPositive_NoSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormUserRepository(db)

	if err := repo.SubStorageUsed(context.Background(), nil, 1, 0); err != nil {
		t.Fatalf("SubStorageUsed with zero delta failed: %v", err)
	}
	if err := repo.SubStorageUsed(context.Background(), nil, 1, -5); err != nil {
		t.Fatalf("SubStorageUsed with negative delta failed: %v", err)
	}

	assertNoSQLCaptured(t, rec)
}

func TestGormUserRepository_SubStorageUsed_Positive_BuildsUpdateSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormUserRepository(db)

	if err := repo.SubStorageUsed(context.Background(), nil, 1, 9); err != nil {
		t.Fatalf("SubStorageUsed failed: %v", err)
	}

	assertLastSQLContains(t, rec, "update `users`", "greatest(storage_used - ?, 0)", "where id = ?")
}
