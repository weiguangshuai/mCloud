package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"mcloud/config"
	"mcloud/models"
	"mcloud/utils"

	"gorm.io/gorm"
)

type fakeTxManager struct{}

func (fakeTxManager) WithTransaction(_ context.Context, fn func(tx *gorm.DB) error) error {
	return fn(nil)
}

type fakeUserRepo struct {
	countByUsername map[string]int64
	usersByID       map[uint]models.User
	usersByName     map[string]models.User
	nextID          uint
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		countByUsername: map[string]int64{},
		usersByID:       map[uint]models.User{},
		usersByName:     map[string]models.User{},
		nextID:          1,
	}
}

func (r *fakeUserRepo) CountByUsername(_ context.Context, username string) (int64, error) {
	if c, ok := r.countByUsername[username]; ok {
		return c, nil
	}
	if _, ok := r.usersByName[username]; ok {
		return 1, nil
	}
	return 0, nil
}

func (r *fakeUserRepo) Create(_ context.Context, _ *gorm.DB, user *models.User) error {
	if user.ID == 0 {
		user.ID = r.nextID
		r.nextID++
	}
	r.usersByID[user.ID] = *user
	r.usersByName[user.Username] = *user
	return nil
}

func (r *fakeUserRepo) GetByUsername(_ context.Context, _ *gorm.DB, username string) (models.User, error) {
	user, ok := r.usersByName[username]
	if !ok {
		return models.User{}, gorm.ErrRecordNotFound
	}
	return user, nil
}

func (r *fakeUserRepo) GetByID(_ context.Context, _ *gorm.DB, userID uint) (models.User, error) {
	user, ok := r.usersByID[userID]
	if !ok {
		return models.User{}, gorm.ErrRecordNotFound
	}
	return user, nil
}

func (r *fakeUserRepo) AddStorageUsed(_ context.Context, _ *gorm.DB, _ uint, _ int64) error {
	return nil
}

func (r *fakeUserRepo) SubStorageUsed(_ context.Context, _ *gorm.DB, _ uint, _ int64) error {
	return nil
}

type fakeFolderRepo struct {
	roots  map[uint]models.Folder
	nextID uint
}

func newFakeFolderRepo() *fakeFolderRepo {
	return &fakeFolderRepo{roots: map[uint]models.Folder{}, nextID: 100}
}

func (r *fakeFolderRepo) GetByIDAndUser(_ context.Context, _ *gorm.DB, folderID uint, userID uint) (models.Folder, error) {
	if root, ok := r.roots[userID]; ok && root.ID == folderID {
		return root, nil
	}
	return models.Folder{}, gorm.ErrRecordNotFound
}

func (r *fakeFolderRepo) GetByIDAndUserUnscoped(ctx context.Context, tx *gorm.DB, folderID uint, userID uint) (models.Folder, error) {
	return r.GetByIDAndUser(ctx, tx, folderID, userID)
}

func (r *fakeFolderRepo) GetRootByUser(_ context.Context, _ *gorm.DB, userID uint) (models.Folder, error) {
	root, ok := r.roots[userID]
	if !ok {
		return models.Folder{}, gorm.ErrRecordNotFound
	}
	return root, nil
}

func (r *fakeFolderRepo) Create(_ context.Context, _ *gorm.DB, folder *models.Folder) error {
	if folder.ID == 0 {
		folder.ID = r.nextID
		r.nextID++
	}
	if folder.IsRoot != nil && *folder.IsRoot {
		r.roots[folder.UserID] = *folder
	}
	return nil
}

func (r *fakeFolderRepo) ListByParent(context.Context, *gorm.DB, uint, uint, bool) ([]models.Folder, error) {
	return nil, errors.New("not implemented")
}

func (r *fakeFolderRepo) CountByParentAndName(context.Context, *gorm.DB, uint, uint, string, uint) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *fakeFolderRepo) UpdateByID(context.Context, *gorm.DB, uint, map[string]interface{}) error {
	return errors.New("not implemented")
}

func (r *fakeFolderRepo) UpdateByIDUnscoped(context.Context, *gorm.DB, uint, map[string]interface{}) error {
	return errors.New("not implemented")
}

func (r *fakeFolderRepo) ListByPathPrefix(context.Context, *gorm.DB, uint, uint, string, bool) ([]models.Folder, error) {
	return nil, errors.New("not implemented")
}

func (r *fakeFolderRepo) PluckIDsByPathPrefix(context.Context, *gorm.DB, uint, uint, string) ([]uint, error) {
	return nil, errors.New("not implemented")
}

func (r *fakeFolderRepo) SoftDeleteByPathPrefix(context.Context, *gorm.DB, uint, uint, string) error {
	return errors.New("not implemented")
}

func (r *fakeFolderRepo) UnscopedDeleteByIDs(context.Context, *gorm.DB, []uint) error {
	return errors.New("not implemented")
}

func TestAuthServiceRegisterSuccess(t *testing.T) {
	config.AppConfig = &config.Config{Storage: config.StorageConfig{DefaultUserQuota: 10 * 1024 * 1024}}

	users := newFakeUserRepo()
	folders := newFakeFolderRepo()
	svc := NewAuthService(fakeTxManager{}, users, folders)

	out, err := svc.Register(context.Background(), RegisterInput{
		Username: "alice",
		Password: "secret123",
		Nickname: "Alice",
	})
	if err != nil {
		t.Fatalf("register returned error: %v", err)
	}
	if out.ID == 0 {
		t.Fatalf("expected user id to be assigned")
	}
	if _, ok := folders.roots[out.ID]; !ok {
		t.Fatalf("expected root folder to be created for user %d", out.ID)
	}
}

func TestAuthServiceRegisterConflict(t *testing.T) {
	config.AppConfig = &config.Config{Storage: config.StorageConfig{DefaultUserQuota: 10 * 1024 * 1024}}

	users := newFakeUserRepo()
	users.countByUsername["taken"] = 1
	folders := newFakeFolderRepo()
	svc := NewAuthService(fakeTxManager{}, users, folders)

	_, err := svc.Register(context.Background(), RegisterInput{
		Username: "taken",
		Password: "secret123",
	})
	if err == nil {
		t.Fatalf("expected conflict error")
	}
	appErr, ok := err.(*AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.HTTPCode != 400 {
		t.Fatalf("expected HTTP 400, got %d", appErr.HTTPCode)
	}
}

func TestAuthServiceLoginWrongPassword(t *testing.T) {
	config.AppConfig = &config.Config{Storage: config.StorageConfig{DefaultUserQuota: 10 * 1024 * 1024}}

	users := newFakeUserRepo()
	folders := newFakeFolderRepo()

	hash, err := utils.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}
	user := models.User{
		ID:           7,
		Username:     "bob",
		Password:     hash,
		Nickname:     "Bob",
		StorageQuota: 1024,
		StorageUsed:  0,
		CreatedAt:    time.Now(),
	}
	users.usersByID[user.ID] = user
	users.usersByName[user.Username] = user

	svc := NewAuthService(fakeTxManager{}, users, folders)
	_, err = svc.Login(context.Background(), LoginInput{Username: "bob", Password: "wrong"})
	if err == nil {
		t.Fatalf("expected unauthorized error")
	}
	appErr, ok := err.(*AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.HTTPCode != 401 {
		t.Fatalf("expected HTTP 401, got %d", appErr.HTTPCode)
	}
}
