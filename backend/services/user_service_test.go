package services

import (
	"context"
	"errors"
	"testing"

	"mcloud/models"

	"gorm.io/gorm"
)

type quotaUserRepo struct {
	*fakeUserRepo
	getByIDErr error
}

func newQuotaUserRepo() *quotaUserRepo {
	return &quotaUserRepo{fakeUserRepo: newFakeUserRepo()}
}

func (r *quotaUserRepo) GetByID(ctx context.Context, tx *gorm.DB, userID uint) (models.User, error) {
	if r.getByIDErr != nil {
		return models.User{}, r.getByIDErr
	}
	return r.fakeUserRepo.GetByID(ctx, tx, userID)
}

func TestUserServiceGetStorageQuotaSuccess(t *testing.T) {
	users := newQuotaUserRepo()
	users.usersByID[10] = models.User{ID: 10, Username: "alice", StorageQuota: 1000, StorageUsed: 250}

	svc := NewUserService(users)
	out, err := svc.GetStorageQuota(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.StorageQuota != 1000 || out.StorageUsed != 250 {
		t.Fatalf("unexpected quota values: %+v", out)
	}
	if out.AvailableSpace != 750 {
		t.Fatalf("expected available space 750, got %d", out.AvailableSpace)
	}
	if out.UsagePercent != 25 {
		t.Fatalf("expected usage percent 25, got %f", out.UsagePercent)
	}
}

func TestUserServiceGetStorageQuotaZeroQuota(t *testing.T) {
	users := newQuotaUserRepo()
	users.usersByID[11] = models.User{ID: 11, Username: "bob", StorageQuota: 0, StorageUsed: 0}

	svc := NewUserService(users)
	out, err := svc.GetStorageQuota(context.Background(), 11)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.UsagePercent != 0 {
		t.Fatalf("expected usage percent 0 for zero quota, got %f", out.UsagePercent)
	}
}

func TestUserServiceGetStorageQuotaNotFound(t *testing.T) {
	users := newQuotaUserRepo()
	users.getByIDErr = gorm.ErrRecordNotFound

	svc := NewUserService(users)
	_, err := svc.GetStorageQuota(context.Background(), 99)
	if err == nil {
		t.Fatalf("expected not found error")
	}

	appErr, ok := err.(*AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.HTTPCode != 404 {
		t.Fatalf("expected HTTP 404, got %d", appErr.HTTPCode)
	}
}

func TestUserServiceGetStorageQuotaInternalError(t *testing.T) {
	users := newQuotaUserRepo()
	users.getByIDErr = errors.New("db timeout")

	svc := NewUserService(users)
	_, err := svc.GetStorageQuota(context.Background(), 99)
	if err == nil {
		t.Fatalf("expected internal error")
	}

	appErr, ok := err.(*AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.HTTPCode != 500 {
		t.Fatalf("expected HTTP 500, got %d", appErr.HTTPCode)
	}
}
