package services

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mcloud/config"
	"mcloud/models"
	"mcloud/repositories"

	"gorm.io/gorm"
)

type trackingUserRepo struct {
	*fakeUserRepo
	addStorageDeltas []int64
}

func newTrackingUserRepo() *trackingUserRepo {
	return &trackingUserRepo{fakeUserRepo: newFakeUserRepo()}
}

func (r *trackingUserRepo) AddStorageUsed(_ context.Context, _ *gorm.DB, userID uint, delta int64) error {
	user, ok := r.usersByID[userID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	user.StorageUsed += delta
	r.usersByID[userID] = user
	r.usersByName[user.Username] = user
	r.addStorageDeltas = append(r.addStorageDeltas, delta)
	return nil
}

func (r *trackingUserRepo) SubStorageUsed(_ context.Context, _ *gorm.DB, userID uint, delta int64) error {
	user, ok := r.usersByID[userID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	user.StorageUsed -= delta
	r.usersByID[userID] = user
	r.usersByName[user.Username] = user
	return nil
}

type fakeFileRepo struct {
	created []models.File
	nextID  uint
}

func newFakeFileRepo() *fakeFileRepo {
	return &fakeFileRepo{nextID: 1}
}

func (r *fakeFileRepo) CountByFolder(context.Context, *gorm.DB, uint, uint, uint, bool) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *fakeFileRepo) CountByFolderAndOriginalName(context.Context, *gorm.DB, uint, uint, string, uint, bool) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *fakeFileRepo) ListByFolder(context.Context, *gorm.DB, repositories.ListFilesInput) ([]models.File, error) {
	return nil, errors.New("not implemented")
}

func (r *fakeFileRepo) ListByFolderIDs(context.Context, *gorm.DB, uint, []uint, bool, bool) ([]models.File, error) {
	return nil, errors.New("not implemented")
}

func (r *fakeFileRepo) Create(_ context.Context, _ *gorm.DB, file *models.File) error {
	if file.ID == 0 {
		file.ID = r.nextID
		r.nextID++
	}
	r.created = append(r.created, *file)
	return nil
}

func (r *fakeFileRepo) GetByIDAndUser(context.Context, *gorm.DB, uint, uint, bool) (models.File, error) {
	return models.File{}, errors.New("not implemented")
}

func (r *fakeFileRepo) GetByIDAndUserUnscoped(context.Context, *gorm.DB, uint, uint, bool) (models.File, error) {
	return models.File{}, errors.New("not implemented")
}

func (r *fakeFileRepo) GetByIDsAndUser(context.Context, *gorm.DB, uint, []uint, bool) ([]models.File, error) {
	return nil, errors.New("not implemented")
}

func (r *fakeFileRepo) UpdateByIDAndUser(context.Context, *gorm.DB, uint, uint, map[string]interface{}) error {
	return errors.New("not implemented")
}

func (r *fakeFileRepo) UpdateByIDsAndUser(context.Context, *gorm.DB, []uint, uint, map[string]interface{}) error {
	return errors.New("not implemented")
}

func (r *fakeFileRepo) SoftDeleteByIDAndUser(context.Context, *gorm.DB, uint, uint) error {
	return errors.New("not implemented")
}

func (r *fakeFileRepo) SoftDeleteByFolderIDs(context.Context, *gorm.DB, uint, []uint) error {
	return errors.New("not implemented")
}

func (r *fakeFileRepo) UnscopedDeleteByIDAndUser(context.Context, *gorm.DB, uint, uint) error {
	return errors.New("not implemented")
}

func (r *fakeFileRepo) UnscopedRestoreByIDAndUser(context.Context, *gorm.DB, uint, uint, map[string]interface{}) error {
	return errors.New("not implemented")
}

func (r *fakeFileRepo) UnscopedRestoreByFolderIDs(context.Context, *gorm.DB, uint, []uint, map[string]interface{}) error {
	return errors.New("not implemented")
}

func (r *fakeFileRepo) FindByUserAndMD5(context.Context, *gorm.DB, uint, string) (models.FileObject, error) {
	return models.FileObject{}, errors.New("not implemented")
}

type fakeFileObjectRepo struct {
	objectsByMD5  map[string]models.FileObject
	getByMD5Err   error
	incrementedID []uint
	createCalled  int
}

func newFakeFileObjectRepo() *fakeFileObjectRepo {
	return &fakeFileObjectRepo{objectsByMD5: map[string]models.FileObject{}}
}

func (r *fakeFileObjectRepo) Create(_ context.Context, _ *gorm.DB, fileObject *models.FileObject) error {
	r.createCalled++
	if fileObject.ID == 0 {
		fileObject.ID = uint(100 + r.createCalled)
	}
	r.objectsByMD5[fileObject.FileMD5] = *fileObject
	return nil
}

func (r *fakeFileObjectRepo) GetByID(_ context.Context, _ *gorm.DB, fileObjectID uint) (models.FileObject, error) {
	for _, obj := range r.objectsByMD5 {
		if obj.ID == fileObjectID {
			return obj, nil
		}
	}
	return models.FileObject{}, gorm.ErrRecordNotFound
}

func (r *fakeFileObjectRepo) GetByMD5(_ context.Context, _ *gorm.DB, value string) (models.FileObject, error) {
	if r.getByMD5Err != nil {
		return models.FileObject{}, r.getByMD5Err
	}
	obj, ok := r.objectsByMD5[value]
	if !ok {
		return models.FileObject{}, gorm.ErrRecordNotFound
	}
	return obj, nil
}

func (r *fakeFileObjectRepo) IncrementRefCount(_ context.Context, _ *gorm.DB, fileObjectID uint) error {
	r.incrementedID = append(r.incrementedID, fileObjectID)
	return nil
}

func (r *fakeFileObjectRepo) DecrementRefCount(context.Context, *gorm.DB, uint) error {
	return nil
}

func (r *fakeFileObjectRepo) DeleteByID(context.Context, *gorm.DB, uint) error {
	return nil
}

type fakeUploadTaskRepo struct {
	tasks map[string]models.UploadTask
}

func newFakeUploadTaskRepo() *fakeUploadTaskRepo {
	return &fakeUploadTaskRepo{tasks: map[string]models.UploadTask{}}
}

func (r *fakeUploadTaskRepo) Create(_ context.Context, _ *gorm.DB, task *models.UploadTask) error {
	r.tasks[task.UploadID] = *task
	return nil
}

func (r *fakeUploadTaskRepo) GetByUploadID(_ context.Context, _ *gorm.DB, uploadID string) (models.UploadTask, error) {
	task, ok := r.tasks[uploadID]
	if !ok {
		return models.UploadTask{}, gorm.ErrRecordNotFound
	}
	return task, nil
}

func (r *fakeUploadTaskRepo) GetByUploadIDAndUser(_ context.Context, _ *gorm.DB, uploadID string, userID uint) (models.UploadTask, error) {
	task, ok := r.tasks[uploadID]
	if !ok || task.UserID != userID {
		return models.UploadTask{}, gorm.ErrRecordNotFound
	}
	return task, nil
}

func (r *fakeUploadTaskRepo) UpdateStatus(_ context.Context, _ *gorm.DB, uploadID string, status string) error {
	task, ok := r.tasks[uploadID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	task.Status = status
	r.tasks[uploadID] = task
	return nil
}

func (r *fakeUploadTaskRepo) DeleteByID(_ context.Context, _ *gorm.DB, id uint) error {
	for uploadID, task := range r.tasks {
		if task.ID == id {
			delete(r.tasks, uploadID)
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (r *fakeUploadTaskRepo) ListExpiredAndUncompleted(_ context.Context, _ *gorm.DB, now time.Time) ([]models.UploadTask, error) {
	out := make([]models.UploadTask, 0)
	for _, task := range r.tasks {
		if task.Status != "completed" && task.ExpiresAt.Before(now) {
			out = append(out, task)
		}
	}
	return out, nil
}

type fakeUploadProgressRepo struct {
	chunks             map[string]map[int]struct{}
	isChunkUploadedErr error
	addChunkErr        error
	uploadedChunksErr  error
}

func newFakeUploadProgressRepo() *fakeUploadProgressRepo {
	return &fakeUploadProgressRepo{chunks: map[string]map[int]struct{}{}}
}

func (r *fakeUploadProgressRepo) IsChunkUploaded(_ context.Context, uploadID string, chunkIndex int) (bool, error) {
	if r.isChunkUploadedErr != nil {
		return false, r.isChunkUploadedErr
	}
	_, ok := r.chunks[uploadID][chunkIndex]
	return ok, nil
}

func (r *fakeUploadProgressRepo) AddChunk(_ context.Context, uploadID string, chunkIndex int, _ int) error {
	if r.addChunkErr != nil {
		return r.addChunkErr
	}
	if _, ok := r.chunks[uploadID]; !ok {
		r.chunks[uploadID] = map[int]struct{}{}
	}
	r.chunks[uploadID][chunkIndex] = struct{}{}
	return nil
}

func (r *fakeUploadProgressRepo) UploadedCount(_ context.Context, uploadID string) (int64, error) {
	return int64(len(r.chunks[uploadID])), nil
}

func (r *fakeUploadProgressRepo) UploadedChunks(_ context.Context, uploadID string) ([]int, error) {
	if r.uploadedChunksErr != nil {
		return nil, r.uploadedChunksErr
	}
	out := make([]int, 0, len(r.chunks[uploadID]))
	for idx := range r.chunks[uploadID] {
		out = append(out, idx)
	}
	return out, nil
}

func (r *fakeUploadProgressRepo) Clear(_ context.Context, uploadID string) error {
	delete(r.chunks, uploadID)
	return nil
}

type testMultipartFile struct {
	*bytes.Reader
}

func (f *testMultipartFile) Close() error {
	return nil
}

func makeMultipartFile(fileName string, content []byte) (multipart.File, *multipart.FileHeader, string) {
	sum := md5.Sum(content)
	fileMD5 := hex.EncodeToString(sum[:])

	header := &multipart.FileHeader{
		Filename: fileName,
		Size:     int64(len(content)),
		Header:   textproto.MIMEHeader{},
	}
	header.Header.Set("Content-Type", "text/plain")

	return &testMultipartFile{Reader: bytes.NewReader(content)}, header, fileMD5
}

func TestFileServiceUploadFileInstantUploadByMD5(t *testing.T) {
	config.AppConfig = &config.Config{
		Storage: config.StorageConfig{
			MaxFileSize:       10 * 1024 * 1024,
			AllowedExtensions: []string{"*"},
		},
	}

	users := newTrackingUserRepo()
	users.usersByID[1] = models.User{ID: 1, Username: "alice", StorageQuota: 1000, StorageUsed: 0}
	users.usersByName["alice"] = users.usersByID[1]

	files := newFakeFileRepo()
	fileObjects := newFakeFileObjectRepo()

	file, header, fileMD5 := makeMultipartFile("hello.txt", []byte("hello world"))
	existing := models.FileObject{
		ID:       7,
		FilePath: "files/shared/object-7.bin",
		FileSize: header.Size,
		FileMD5:  fileMD5,
	}
	fileObjects.objectsByMD5[fileMD5] = existing

	svc := NewFileService(fakeTxManager{}, users, newFakeFolderRepo(), files, fileObjects, nil, nil, nil)
	out, err := svc.UploadFile(context.Background(), 1, 0, file, header)
	if err != nil {
		t.Fatalf("UploadFile returned error: %v", err)
	}

	if out.FileObjectID != existing.ID {
		t.Fatalf("expected FileObjectID %d, got %d", existing.ID, out.FileObjectID)
	}
	if out.Name != "object-7.bin" {
		t.Fatalf("expected stored name object-7.bin, got %s", out.Name)
	}
	if out.FileObject.ID != existing.ID {
		t.Fatalf("expected embedded FileObject ID %d, got %d", existing.ID, out.FileObject.ID)
	}
	if len(fileObjects.incrementedID) != 1 || fileObjects.incrementedID[0] != existing.ID {
		t.Fatalf("expected ref count increment for object %d", existing.ID)
	}
	if len(files.created) != 1 {
		t.Fatalf("expected one logical file record, got %d", len(files.created))
	}
	if users.usersByID[1].StorageUsed != header.Size {
		t.Fatalf("expected storage used %d, got %d", header.Size, users.usersByID[1].StorageUsed)
	}
	if fileObjects.createCalled != 0 {
		t.Fatalf("expected no new file object creation, got %d", fileObjects.createCalled)
	}
}

func TestFileServiceUploadFileDuplicateCheckFailure(t *testing.T) {
	config.AppConfig = &config.Config{
		Storage: config.StorageConfig{
			MaxFileSize:       10 * 1024 * 1024,
			AllowedExtensions: []string{"*"},
		},
	}

	users := newTrackingUserRepo()
	users.usersByID[1] = models.User{ID: 1, Username: "alice", StorageQuota: 1000, StorageUsed: 0}
	users.usersByName["alice"] = users.usersByID[1]

	files := newFakeFileRepo()
	fileObjects := newFakeFileObjectRepo()
	fileObjects.getByMD5Err = errors.New("db unavailable")

	file, header, _ := makeMultipartFile("hello.txt", []byte("hello world"))
	svc := NewFileService(fakeTxManager{}, users, newFakeFolderRepo(), files, fileObjects, nil, nil, nil)
	_, err := svc.UploadFile(context.Background(), 1, 0, file, header)
	if err == nil {
		t.Fatalf("expected UploadFile to return error")
	}

	appErr, ok := err.(*AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.HTTPCode != http.StatusInternalServerError {
		t.Fatalf("expected HTTP 500, got %d", appErr.HTTPCode)
	}
	if appErr.Message != "failed to check duplicate file" {
		t.Fatalf("unexpected error message: %s", appErr.Message)
	}
	if len(files.created) != 0 {
		t.Fatalf("expected no file record to be created")
	}
}

func TestFileServiceInitChunkedUploadInstantUploadByMD5(t *testing.T) {
	config.AppConfig = &config.Config{
		Storage: config.StorageConfig{
			AllowedExtensions: []string{"*"},
			ChunkSize:         5 * 1024 * 1024,
		},
	}

	users := newTrackingUserRepo()
	users.usersByID[1] = models.User{ID: 1, Username: "alice", StorageQuota: 1 << 30, StorageUsed: 0}
	users.usersByName["alice"] = users.usersByID[1]

	files := newFakeFileRepo()
	fileObjects := newFakeFileObjectRepo()

	fileMD5 := "0123456789abcdef0123456789abcdef"
	existing := models.FileObject{
		ID:       11,
		FilePath: "files/shared/existing.bin",
		FileSize: 12345,
		FileMD5:  fileMD5,
	}
	fileObjects.objectsByMD5[fileMD5] = existing

	svc := NewFileService(fakeTxManager{}, users, newFakeFolderRepo(), files, fileObjects, nil, nil, nil)
	out, err := svc.InitChunkedUpload(context.Background(), 1, InitChunkedUploadInput{
		FileName: "movie.mp4",
		FileSize: existing.FileSize,
		FileMD5:  fileMD5,
		FolderID: 0,
	})
	if err != nil {
		t.Fatalf("InitChunkedUpload returned error: %v", err)
	}

	if out.Status != "instant_upload" {
		t.Fatalf("expected instant_upload, got %s", out.Status)
	}
	if out.FileID == 0 {
		t.Fatalf("expected a created logical file ID")
	}
	if len(fileObjects.incrementedID) != 1 || fileObjects.incrementedID[0] != existing.ID {
		t.Fatalf("expected ref count increment for object %d", existing.ID)
	}
	if len(files.created) != 1 {
		t.Fatalf("expected one created logical file, got %d", len(files.created))
	}
	if files.created[0].FileObjectID != existing.ID {
		t.Fatalf("expected created file to reference object %d, got %d", existing.ID, files.created[0].FileObjectID)
	}
	if len(users.addStorageDeltas) != 1 || users.addStorageDeltas[0] != existing.FileSize {
		t.Fatalf("expected storage increment %d, got %#v", existing.FileSize, users.addStorageDeltas)
	}
}

func TestFileServiceUploadChunkUsesDiskFallbackWhenProgressStoreFails(t *testing.T) {
	baseDir := t.TempDir()
	task := models.UploadTask{
		ID:          1,
		UploadID:    "upload-1",
		UserID:      1,
		TotalChunks: 3,
		TempDir:     filepath.Join(baseDir, "temp", "upload-1"),
		Status:      "uploading",
	}

	uploadTasks := newFakeUploadTaskRepo()
	uploadTasks.tasks[task.UploadID] = task

	uploadProgress := newFakeUploadProgressRepo()
	uploadProgress.isChunkUploadedErr = errors.New("redis unavailable")
	uploadProgress.addChunkErr = errors.New("redis unavailable")
	uploadProgress.uploadedChunksErr = errors.New("redis unavailable")

	config.AppConfig = &config.Config{
		Redis: config.RedisConfig{
			UploadTaskExpire: 86400,
		},
	}

	svc := NewFileService(
		fakeTxManager{},
		newTrackingUserRepo(),
		newFakeFolderRepo(),
		newFakeFileRepo(),
		newFakeFileObjectRepo(),
		uploadTasks,
		nil,
		uploadProgress,
	)

	chunkA, _, _ := makeMultipartFile("chunk.bin", []byte("part-a"))
	out, err := svc.UploadChunk(context.Background(), 1, task.UploadID, 0, chunkA)
	if err != nil {
		t.Fatalf("UploadChunk returned error: %v", err)
	}
	if out.UploadedChunks != 1 {
		t.Fatalf("expected uploaded chunk count 1, got %d", out.UploadedChunks)
	}
	if !chunkFileExists(task.TempDir, 0) {
		t.Fatalf("expected chunk file to exist on disk")
	}

	chunkB, _, _ := makeMultipartFile("chunk.bin", []byte("part-a"))
	out, err = svc.UploadChunk(context.Background(), 1, task.UploadID, 0, chunkB)
	if err != nil {
		t.Fatalf("re-upload chunk returned error: %v", err)
	}
	if out.Message != "分片已存在" {
		t.Fatalf("expected duplicate chunk message, got %q", out.Message)
	}

	chunkPath := chunkFilePath(task.TempDir, 0)
	if _, err := os.Stat(chunkPath); err != nil {
		t.Fatalf("expected chunk path %s to exist: %v", chunkPath, err)
	}
}
