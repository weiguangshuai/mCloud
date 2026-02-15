package services

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"mcloud/config"
	"mcloud/models"
	"mcloud/repositories"

	"gorm.io/gorm"
)

type folderServiceFolderRepo struct {
	*fakeFolderRepo
	folders             map[uint]models.Folder
	rootByUser          map[uint]uint
	nextID              uint
	getByIDErr          error
	getRootErr          error
	createErr           error
	listByParentErr     error
	countErr            error
	updateErr           error
	listByPathPrefixErr error
	pluckErr            error
	softDeleteErr       error
	lastListByParent    struct {
		userID            uint
		parentID          uint
		includeLegacyRoot bool
	}
	softDeleteCalls []struct {
		userID   uint
		rootID   uint
		rootPath string
	}
}

func newFolderServiceFolderRepo() *folderServiceFolderRepo {
	return &folderServiceFolderRepo{
		fakeFolderRepo: newFakeFolderRepo(),
		folders:        map[uint]models.Folder{},
		rootByUser:     map[uint]uint{},
		nextID:         1,
	}
}

func (r *folderServiceFolderRepo) GetByIDAndUser(_ context.Context, _ *gorm.DB, folderID uint, userID uint) (models.Folder, error) {
	if r.getByIDErr != nil {
		return models.Folder{}, r.getByIDErr
	}
	folder, ok := r.folders[folderID]
	if !ok || folder.UserID != userID {
		return models.Folder{}, gorm.ErrRecordNotFound
	}
	return folder, nil
}

func (r *folderServiceFolderRepo) GetByIDAndUserUnscoped(ctx context.Context, tx *gorm.DB, folderID uint, userID uint) (models.Folder, error) {
	return r.GetByIDAndUser(ctx, tx, folderID, userID)
}

func (r *folderServiceFolderRepo) GetRootByUser(_ context.Context, _ *gorm.DB, userID uint) (models.Folder, error) {
	if r.getRootErr != nil {
		return models.Folder{}, r.getRootErr
	}
	rootID, ok := r.rootByUser[userID]
	if !ok {
		return models.Folder{}, gorm.ErrRecordNotFound
	}
	return r.folders[rootID], nil
}

func (r *folderServiceFolderRepo) Create(_ context.Context, _ *gorm.DB, folder *models.Folder) error {
	if r.createErr != nil {
		return r.createErr
	}
	if folder.ID == 0 {
		folder.ID = r.nextID
		r.nextID++
	}
	copied := *folder
	r.folders[folder.ID] = copied
	if folder.IsRoot != nil && *folder.IsRoot {
		r.rootByUser[folder.UserID] = folder.ID
	}
	return nil
}

func (r *folderServiceFolderRepo) ListByParent(_ context.Context, _ *gorm.DB, userID uint, parentID uint, includeLegacyRoot bool) ([]models.Folder, error) {
	if r.listByParentErr != nil {
		return nil, r.listByParentErr
	}
	r.lastListByParent = struct {
		userID            uint
		parentID          uint
		includeLegacyRoot bool
	}{userID: userID, parentID: parentID, includeLegacyRoot: includeLegacyRoot}

	out := make([]models.Folder, 0)
	for _, folder := range r.folders {
		if folder.UserID != userID || folder.ParentID == nil {
			continue
		}
		if *folder.ParentID == parentID {
			out = append(out, folder)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *folderServiceFolderRepo) CountByParentAndName(_ context.Context, _ *gorm.DB, userID uint, parentID uint, name string, excludeID uint) (int64, error) {
	if r.countErr != nil {
		return 0, r.countErr
	}
	var count int64
	for _, folder := range r.folders {
		if folder.UserID != userID || folder.ID == excludeID || folder.Name != name {
			continue
		}
		currentParent := uint(0)
		if folder.ParentID != nil {
			currentParent = *folder.ParentID
		}
		if currentParent == parentID {
			count++
		}
	}
	return count, nil
}

func (r *folderServiceFolderRepo) UpdateByID(_ context.Context, _ *gorm.DB, folderID uint, updates map[string]interface{}) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	folder, ok := r.folders[folderID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	for key, value := range updates {
		switch key {
		case "name":
			folder.Name = value.(string)
		case "path":
			folder.Path = value.(string)
		case "parent_id":
			parentID := value.(uint)
			folder.ParentID = &parentID
		}
	}
	r.folders[folderID] = folder
	return nil
}

func (r *folderServiceFolderRepo) UpdateByIDUnscoped(ctx context.Context, tx *gorm.DB, folderID uint, updates map[string]interface{}) error {
	return r.UpdateByID(ctx, tx, folderID, updates)
}

func (r *folderServiceFolderRepo) ListByPathPrefix(_ context.Context, _ *gorm.DB, userID uint, rootID uint, rootPath string, _ bool) ([]models.Folder, error) {
	if r.listByPathPrefixErr != nil {
		return nil, r.listByPathPrefixErr
	}
	out := make([]models.Folder, 0)
	for _, folder := range r.folders {
		if folder.UserID != userID {
			continue
		}
		if folder.ID == rootID || folder.Path == rootPath || strings.HasPrefix(folder.Path, rootPath+"/") {
			out = append(out, folder)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *folderServiceFolderRepo) PluckIDsByPathPrefix(_ context.Context, _ *gorm.DB, userID uint, rootID uint, rootPath string) ([]uint, error) {
	if r.pluckErr != nil {
		return nil, r.pluckErr
	}
	out := make([]uint, 0)
	for _, folder := range r.folders {
		if folder.UserID != userID {
			continue
		}
		if folder.ID == rootID || folder.Path == rootPath || strings.HasPrefix(folder.Path, rootPath+"/") {
			out = append(out, folder.ID)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

func (r *folderServiceFolderRepo) SoftDeleteByPathPrefix(_ context.Context, _ *gorm.DB, userID uint, rootID uint, rootPath string) error {
	if r.softDeleteErr != nil {
		return r.softDeleteErr
	}
	r.softDeleteCalls = append(r.softDeleteCalls, struct {
		userID   uint
		rootID   uint
		rootPath string
	}{userID: userID, rootID: rootID, rootPath: rootPath})
	return nil
}

type folderServiceFileRepo struct {
	*fakeFileRepo
	softDeleteErr   error
	softDeleteCalls []struct {
		userID    uint
		folderIDs []uint
	}
}

func newFolderServiceFileRepo() *folderServiceFileRepo {
	return &folderServiceFileRepo{fakeFileRepo: newFakeFileRepo()}
}

func (r *folderServiceFileRepo) SoftDeleteByFolderIDs(_ context.Context, _ *gorm.DB, userID uint, folderIDs []uint) error {
	if r.softDeleteErr != nil {
		return r.softDeleteErr
	}
	copied := append([]uint(nil), folderIDs...)
	r.softDeleteCalls = append(r.softDeleteCalls, struct {
		userID    uint
		folderIDs []uint
	}{userID: userID, folderIDs: copied})
	return nil
}

type folderServiceRecycleRepo struct {
	createErr error
	items     []models.RecycleBinItem
}

func (r *folderServiceRecycleRepo) CountByUser(context.Context, *gorm.DB, uint) (int64, error) {
	return 0, nil
}
func (r *folderServiceRecycleRepo) ListByUser(context.Context, *gorm.DB, repositories.RecycleBinListInput) ([]models.RecycleBinItem, error) {
	return nil, nil
}
func (r *folderServiceRecycleRepo) ListAllByUser(context.Context, *gorm.DB, uint) ([]models.RecycleBinItem, error) {
	return nil, nil
}
func (r *folderServiceRecycleRepo) ListExpired(context.Context, *gorm.DB, time.Time) ([]models.RecycleBinItem, error) {
	return nil, nil
}
func (r *folderServiceRecycleRepo) GetByIDAndUser(context.Context, *gorm.DB, uint, uint) (models.RecycleBinItem, error) {
	return models.RecycleBinItem{}, gorm.ErrRecordNotFound
}
func (r *folderServiceRecycleRepo) Create(_ context.Context, _ *gorm.DB, item *models.RecycleBinItem) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.items = append(r.items, *item)
	return nil
}
func (r *folderServiceRecycleRepo) DeleteByID(context.Context, *gorm.DB, uint) error { return nil }
func (r *folderServiceRecycleRepo) DeleteByUser(context.Context, *gorm.DB, uint) error {
	return nil
}
func (r *folderServiceRecycleRepo) DeleteByOriginalIDs(context.Context, *gorm.DB, uint, string, []uint) error {
	return nil
}

func TestFolderServiceResolveFolderIDNotFound(t *testing.T) {
	repo := newFolderServiceFolderRepo()
	repo.getByIDErr = gorm.ErrRecordNotFound

	svc := NewFolderService(fakeTxManager{}, repo, newFolderServiceFileRepo(), &folderServiceRecycleRepo{})
	_, err := svc.ResolveFolderID(context.Background(), 1, 123)
	if err == nil {
		t.Fatalf("expected error")
	}

	appErr, ok := err.(*AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.HTTPCode != http.StatusNotFound {
		t.Fatalf("expected HTTP 404, got %d", appErr.HTTPCode)
	}
}

func TestFolderServiceCreateFolderSuccess(t *testing.T) {
	repo := newFolderServiceFolderRepo()
	isRoot := true
	repo.folders[1] = models.Folder{ID: 1, Name: "root", UserID: 1, Path: "/", IsRoot: &isRoot}
	repo.rootByUser[1] = 1
	repo.nextID = 2

	svc := NewFolderService(fakeTxManager{}, repo, newFolderServiceFileRepo(), &folderServiceRecycleRepo{})
	folder, err := svc.CreateFolder(context.Background(), 1, "docs", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if folder.ID != 2 {
		t.Fatalf("expected folder id 2, got %d", folder.ID)
	}
	if folder.Path != "/docs" {
		t.Fatalf("expected path /docs, got %s", folder.Path)
	}
	if folder.ParentID == nil || *folder.ParentID != 1 {
		t.Fatalf("expected parent id 1, got %+v", folder.ParentID)
	}
}

func TestFolderServiceRenameFolderUpdatesDescendantPath(t *testing.T) {
	repo := newFolderServiceFolderRepo()
	isRoot := true
	rootID := uint(1)
	parentID := uint(2)
	repo.folders[rootID] = models.Folder{ID: rootID, Name: "root", UserID: 1, Path: "/", IsRoot: &isRoot}
	repo.rootByUser[1] = rootID
	repo.folders[parentID] = models.Folder{ID: parentID, Name: "old", UserID: 1, ParentID: &rootID, Path: "/old"}
	repo.folders[3] = models.Folder{ID: 3, Name: "sub", UserID: 1, ParentID: &parentID, Path: "/old/sub"}

	svc := NewFolderService(fakeTxManager{}, repo, newFolderServiceFileRepo(), &folderServiceRecycleRepo{})
	renamed, err := svc.RenameFolder(context.Background(), 1, parentID, "new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if renamed.Path != "/new" {
		t.Fatalf("expected renamed path /new, got %s", renamed.Path)
	}
	if got := repo.folders[3].Path; got != "/new/sub" {
		t.Fatalf("expected descendant path /new/sub, got %s", got)
	}
}

func TestFolderServiceDeleteFolderWithRecycleBin(t *testing.T) {
	config.AppConfig = &config.Config{RecycleBin: config.RecycleBinConfig{Enabled: true, RetentionDays: 7}}

	repo := newFolderServiceFolderRepo()
	isRoot := true
	rootID := uint(1)
	targetID := uint(2)
	repo.folders[rootID] = models.Folder{ID: rootID, Name: "root", UserID: 1, Path: "/", IsRoot: &isRoot}
	repo.rootByUser[1] = rootID
	repo.folders[targetID] = models.Folder{ID: targetID, Name: "docs", UserID: 1, ParentID: &rootID, Path: "/docs"}
	repo.folders[3] = models.Folder{ID: 3, Name: "sub", UserID: 1, ParentID: &targetID, Path: "/docs/sub"}

	files := newFolderServiceFileRepo()
	recycle := &folderServiceRecycleRepo{}
	svc := NewFolderService(fakeTxManager{}, repo, files, recycle)

	if err := svc.DeleteFolder(context.Background(), 1, targetID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(recycle.items) != 1 {
		t.Fatalf("expected 1 recycle item, got %d", len(recycle.items))
	}
	if recycle.items[0].OriginalID != targetID || recycle.items[0].OriginalType != "folder" {
		t.Fatalf("unexpected recycle item: %+v", recycle.items[0])
	}
	if len(repo.softDeleteCalls) != 1 {
		t.Fatalf("expected folder soft-delete call")
	}
	if len(files.softDeleteCalls) != 1 {
		t.Fatalf("expected file soft-delete call")
	}
	if got := files.softDeleteCalls[0].folderIDs; len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("unexpected affected folder ids: %#v", got)
	}
}

func TestFolderServiceDeleteRootFolderRejected(t *testing.T) {
	repo := newFolderServiceFolderRepo()
	isRoot := true
	repo.folders[1] = models.Folder{ID: 1, Name: "root", UserID: 1, Path: "/", IsRoot: &isRoot}
	repo.rootByUser[1] = 1

	svc := NewFolderService(fakeTxManager{}, repo, newFolderServiceFileRepo(), &folderServiceRecycleRepo{})
	err := svc.DeleteFolder(context.Background(), 1, 1)
	if err == nil {
		t.Fatalf("expected error")
	}
	appErr, ok := err.(*AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.HTTPCode != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400, got %d", appErr.HTTPCode)
	}
}

func TestFolderServiceListFoldersUsesLegacyRootFlagForRoot(t *testing.T) {
	repo := newFolderServiceFolderRepo()
	isRoot := true
	rootID := uint(1)
	repo.folders[rootID] = models.Folder{ID: rootID, Name: "root", UserID: 1, Path: "/", IsRoot: &isRoot}
	repo.rootByUser[1] = rootID
	repo.folders[2] = models.Folder{ID: 2, Name: "docs", UserID: 1, ParentID: &rootID, Path: "/docs"}

	svc := NewFolderService(fakeTxManager{}, repo, newFolderServiceFileRepo(), &folderServiceRecycleRepo{})
	list, err := svc.ListFolders(context.Background(), 1, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 1 || list[0].ID != 2 {
		t.Fatalf("unexpected folder list: %#v", list)
	}
	if !repo.lastListByParent.includeLegacyRoot {
		t.Fatalf("expected includeLegacyRoot=true for root listing")
	}
}

func TestFolderServiceCreateFolderDuplicateName(t *testing.T) {
	repo := newFolderServiceFolderRepo()
	isRoot := true
	rootID := uint(1)
	repo.folders[rootID] = models.Folder{ID: rootID, Name: "root", UserID: 1, Path: "/", IsRoot: &isRoot}
	repo.rootByUser[1] = rootID
	repo.folders[2] = models.Folder{ID: 2, Name: "docs", UserID: 1, ParentID: &rootID, Path: "/docs"}

	svc := NewFolderService(fakeTxManager{}, repo, newFolderServiceFileRepo(), &folderServiceRecycleRepo{})
	_, err := svc.CreateFolder(context.Background(), 1, "docs", 0)
	if err == nil {
		t.Fatalf("expected duplicate-name error")
	}
	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.HTTPCode != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400, got %d", appErr.HTTPCode)
	}
}
