package services

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mcloud/config"
	"mcloud/models"
	"mcloud/repositories"

	"gorm.io/gorm"
)

type recycleServiceRecycleRepo struct {
	count         int64
	countErr      error
	listItems     []models.RecycleBinItem
	listErr       error
	lastListInput repositories.RecycleBinListInput
	getErr        error
}

func (r *recycleServiceRecycleRepo) CountByUser(context.Context, *gorm.DB, uint) (int64, error) {
	if r.countErr != nil {
		return 0, r.countErr
	}
	return r.count, nil
}

func (r *recycleServiceRecycleRepo) ListByUser(_ context.Context, _ *gorm.DB, in repositories.RecycleBinListInput) ([]models.RecycleBinItem, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	r.lastListInput = in
	return append([]models.RecycleBinItem(nil), r.listItems...), nil
}

func (r *recycleServiceRecycleRepo) ListAllByUser(context.Context, *gorm.DB, uint) ([]models.RecycleBinItem, error) {
	return nil, nil
}

func (r *recycleServiceRecycleRepo) ListExpired(context.Context, *gorm.DB, time.Time) ([]models.RecycleBinItem, error) {
	return nil, nil
}

func (r *recycleServiceRecycleRepo) GetByIDAndUser(context.Context, *gorm.DB, uint, uint) (models.RecycleBinItem, error) {
	if r.getErr != nil {
		return models.RecycleBinItem{}, r.getErr
	}
	return models.RecycleBinItem{}, gorm.ErrRecordNotFound
}

func (r *recycleServiceRecycleRepo) Create(context.Context, *gorm.DB, *models.RecycleBinItem) error {
	return nil
}
func (r *recycleServiceRecycleRepo) DeleteByID(context.Context, *gorm.DB, uint) error { return nil }
func (r *recycleServiceRecycleRepo) DeleteByUser(context.Context, *gorm.DB, uint) error {
	return nil
}
func (r *recycleServiceRecycleRepo) DeleteByOriginalIDs(context.Context, *gorm.DB, uint, string, []uint) error {
	return nil
}

type recycleTrackingFileObjectRepo struct {
	*fakeFileObjectRepo
	decrementedIDs []uint
	deletedIDs     []uint
}

func newRecycleTrackingFileObjectRepo() *recycleTrackingFileObjectRepo {
	return &recycleTrackingFileObjectRepo{fakeFileObjectRepo: newFakeFileObjectRepo()}
}

func (r *recycleTrackingFileObjectRepo) DecrementRefCount(_ context.Context, _ *gorm.DB, fileObjectID uint) error {
	r.decrementedIDs = append(r.decrementedIDs, fileObjectID)
	return nil
}

func (r *recycleTrackingFileObjectRepo) DeleteByID(_ context.Context, _ *gorm.DB, fileObjectID uint) error {
	r.deletedIDs = append(r.deletedIDs, fileObjectID)
	for key, obj := range r.objectsByMD5 {
		if obj.ID == fileObjectID {
			delete(r.objectsByMD5, key)
		}
	}
	return nil
}

func TestRecycleBinServiceListRecycleBinNormalizesPagination(t *testing.T) {
	recycleRepo := &recycleServiceRecycleRepo{
		count: 45,
		listItems: []models.RecycleBinItem{
			{ID: 1, UserID: 8, OriginalType: "file"},
		},
	}

	svc := NewRecycleBinService(
		fakeTxManager{},
		newFakeUserRepo(),
		newFakeFolderRepo(),
		newFakeFileRepo(),
		newFakeFileObjectRepo(),
		recycleRepo,
	)

	out, err := svc.ListRecycleBin(context.Background(), 8, 0, 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Pagination.Page != 1 || out.Pagination.PageSize != 20 {
		t.Fatalf("expected normalized pagination 1/20, got %d/%d", out.Pagination.Page, out.Pagination.PageSize)
	}
	if out.Pagination.TotalPages != 3 || !out.Pagination.HasNext || out.Pagination.HasPrev {
		t.Fatalf("unexpected pagination output: %+v", out.Pagination)
	}
	if recycleRepo.lastListInput.Offset != 0 || recycleRepo.lastListInput.Limit != 20 {
		t.Fatalf("unexpected list input: %+v", recycleRepo.lastListInput)
	}
	if recycleRepo.lastListInput.SortSQL != "deleted_at DESC" {
		t.Fatalf("unexpected sort sql: %s", recycleRepo.lastListInput.SortSQL)
	}
}

func TestRecycleBinServiceRestoreItemNotFound(t *testing.T) {
	recycleRepo := &recycleServiceRecycleRepo{getErr: gorm.ErrRecordNotFound}
	svc := NewRecycleBinService(
		fakeTxManager{},
		newFakeUserRepo(),
		newFakeFolderRepo(),
		newFakeFileRepo(),
		newFakeFileObjectRepo(),
		recycleRepo,
	)

	err := svc.RestoreItem(context.Background(), 1, 99)
	if err == nil {
		t.Fatalf("expected not found error")
	}
	appErr, ok := err.(*AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.HTTPCode != http.StatusNotFound {
		t.Fatalf("expected HTTP 404, got %d", appErr.HTTPCode)
	}
}

func TestRecycleBinServiceDecrementFileObjectRefDeletesFilesForLastRef(t *testing.T) {
	baseDir := t.TempDir()
	filePath := filepath.Join(baseDir, "objects", "o-1.bin")
	thumbPath := filepath.Join(baseDir, "thumbs", "o-1.jpg")
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("mkdir file dir failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(thumbPath), 0o755); err != nil {
		t.Fatalf("mkdir thumb dir failed: %v", err)
	}
	if err := os.WriteFile(filePath, []byte("data"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	if err := os.WriteFile(thumbPath, []byte("thumb"), 0o644); err != nil {
		t.Fatalf("write thumb failed: %v", err)
	}

	config.AppConfig = &config.Config{Storage: config.StorageConfig{BasePath: baseDir}}

	fileObjects := newRecycleTrackingFileObjectRepo()
	fileObjects.objectsByMD5["md5"] = models.FileObject{
		ID:            10,
		FilePath:      "objects/o-1.bin",
		ThumbnailPath: "thumbs/o-1.jpg",
		RefCount:      1,
	}

	svc := &recycleBinService{fileObjects: fileObjects}
	if err := svc.decrementFileObjectRef(context.Background(), nil, 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fileObjects.deletedIDs) != 1 || fileObjects.deletedIDs[0] != 10 {
		t.Fatalf("expected delete-by-id for object 10, got %#v", fileObjects.deletedIDs)
	}
	if len(fileObjects.decrementedIDs) != 0 {
		t.Fatalf("expected no ref decrement, got %#v", fileObjects.decrementedIDs)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("expected file to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(thumbPath); !os.IsNotExist(err) {
		t.Fatalf("expected thumbnail to be removed, stat err=%v", err)
	}
}

func TestRecycleBinServiceDecrementFileObjectRefDecrementsWhenShared(t *testing.T) {
	config.AppConfig = &config.Config{Storage: config.StorageConfig{BasePath: t.TempDir()}}

	fileObjects := newRecycleTrackingFileObjectRepo()
	fileObjects.objectsByMD5["md5"] = models.FileObject{
		ID:       11,
		FilePath: "objects/o-2.bin",
		RefCount: 2,
	}

	svc := &recycleBinService{fileObjects: fileObjects}
	if err := svc.decrementFileObjectRef(context.Background(), nil, 11); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fileObjects.decrementedIDs) != 1 || fileObjects.decrementedIDs[0] != 11 {
		t.Fatalf("expected ref decrement for object 11, got %#v", fileObjects.decrementedIDs)
	}
	if len(fileObjects.deletedIDs) != 0 {
		t.Fatalf("expected no object delete, got %#v", fileObjects.deletedIDs)
	}
}
