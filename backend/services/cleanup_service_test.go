package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mcloud/config"
	"mcloud/models"

	"gorm.io/gorm"
)

type cleanupServiceFileRepo struct {
	*fakeFileRepo
	fileByID       map[uint]models.File
	getUnscopedErr error
	deleteErr      error
	deletedIDs     []uint
}

func newCleanupServiceFileRepo() *cleanupServiceFileRepo {
	return &cleanupServiceFileRepo{
		fakeFileRepo: newFakeFileRepo(),
		fileByID:     map[uint]models.File{},
	}
}

func (r *cleanupServiceFileRepo) GetByIDAndUserUnscoped(_ context.Context, _ *gorm.DB, fileID uint, userID uint, _ bool) (models.File, error) {
	if r.getUnscopedErr != nil {
		return models.File{}, r.getUnscopedErr
	}
	file, ok := r.fileByID[fileID]
	if !ok || file.UserID != userID {
		return models.File{}, gorm.ErrRecordNotFound
	}
	return file, nil
}

func (r *cleanupServiceFileRepo) UnscopedDeleteByIDAndUser(_ context.Context, _ *gorm.DB, fileID uint, _ uint) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletedIDs = append(r.deletedIDs, fileID)
	delete(r.fileByID, fileID)
	return nil
}

func TestCleanupServiceCleanExpiredUploadTasksRemovesTempFilesAndTask(t *testing.T) {
	baseDir := t.TempDir()
	expiredDir := filepath.Join(baseDir, "task-expired")
	if err := os.MkdirAll(expiredDir, 0o755); err != nil {
		t.Fatalf("mkdir expired dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(expiredDir, "chunk-0"), []byte("part"), 0o644); err != nil {
		t.Fatalf("write chunk failed: %v", err)
	}

	uploadTasks := newFakeUploadTaskRepo()
	uploadTasks.tasks["expired"] = models.UploadTask{
		ID:        1,
		UploadID:  "expired",
		UserID:    1,
		Status:    "uploading",
		TempDir:   expiredDir,
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	uploadTasks.tasks["completed"] = models.UploadTask{
		ID:        2,
		UploadID:  "completed",
		UserID:    1,
		Status:    "completed",
		TempDir:   filepath.Join(baseDir, "task-completed"),
		ExpiresAt: time.Now().Add(-time.Hour),
	}

	svc := &cleanupService{uploadTasks: uploadTasks}
	svc.cleanExpiredUploadTasks(context.Background())

	if _, ok := uploadTasks.tasks["expired"]; ok {
		t.Fatalf("expected expired task to be removed")
	}
	if _, ok := uploadTasks.tasks["completed"]; !ok {
		t.Fatalf("completed task should not be deleted")
	}
	if _, err := os.Stat(expiredDir); !os.IsNotExist(err) {
		t.Fatalf("expected expired temp dir to be removed, stat err=%v", err)
	}
}

func TestCleanupServiceCleanupPermanentDeleteFileUsesRecycleMetadata(t *testing.T) {
	users := newTrackingUserRepo()
	users.usersByID[1] = models.User{ID: 1, Username: "alice", StorageQuota: 1000, StorageUsed: 100}
	users.usersByName["alice"] = users.usersByID[1]

	files := newCleanupServiceFileRepo()
	files.getUnscopedErr = gorm.ErrRecordNotFound

	fileObjects := newRecycleTrackingFileObjectRepo()
	fileObjects.objectsByMD5["md5"] = models.FileObject{ID: 8, FilePath: "objects/o-8.bin", RefCount: 2}

	size := int64(30)
	fileObjectID := uint(8)
	item := &models.RecycleBinItem{
		UserID:       1,
		OriginalID:   99,
		FileObjectID: &fileObjectID,
		FileSize:     &size,
	}

	svc := &cleanupService{users: users, files: files, fileObjects: fileObjects}
	if err := svc.cleanupPermanentDeleteFile(context.Background(), nil, item); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if users.usersByID[1].StorageUsed != 70 {
		t.Fatalf("expected storage used 70, got %d", users.usersByID[1].StorageUsed)
	}
	if len(files.deletedIDs) != 1 || files.deletedIDs[0] != 99 {
		t.Fatalf("expected logical file delete for id 99, got %#v", files.deletedIDs)
	}
	if len(fileObjects.decrementedIDs) != 1 || fileObjects.decrementedIDs[0] != 8 {
		t.Fatalf("expected ref decrement for file object 8, got %#v", fileObjects.decrementedIDs)
	}
}

func TestCleanupServiceCleanupDecrementFileObjectRefDeletesFilesForLastRef(t *testing.T) {
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
		ID:            11,
		FilePath:      "objects/o-1.bin",
		ThumbnailPath: "thumbs/o-1.jpg",
		RefCount:      1,
	}

	svc := &cleanupService{fileObjects: fileObjects}
	if err := svc.cleanupDecrementFileObjectRef(context.Background(), nil, 11); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fileObjects.deletedIDs) != 1 || fileObjects.deletedIDs[0] != 11 {
		t.Fatalf("expected delete-by-id for object 11, got %#v", fileObjects.deletedIDs)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("expected file to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(thumbPath); !os.IsNotExist(err) {
		t.Fatalf("expected thumbnail to be removed, stat err=%v", err)
	}
}
