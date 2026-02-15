package services

import (
	"context"
	"errors"
	"testing"

	"mcloud/models"

	"gorm.io/gorm"
)

type resolverFolderRepo struct {
	*fakeFolderRepo
	foldersByID map[uint]models.Folder
	getRootErr  error
	getByIDErr  error
	createErr   error
}

func newResolverFolderRepo() *resolverFolderRepo {
	return &resolverFolderRepo{
		fakeFolderRepo: newFakeFolderRepo(),
		foldersByID:    map[uint]models.Folder{},
	}
}

func (r *resolverFolderRepo) GetRootByUser(ctx context.Context, tx *gorm.DB, userID uint) (models.Folder, error) {
	if r.getRootErr != nil {
		return models.Folder{}, r.getRootErr
	}
	return r.fakeFolderRepo.GetRootByUser(ctx, tx, userID)
}

func (r *resolverFolderRepo) GetByIDAndUser(ctx context.Context, tx *gorm.DB, folderID uint, userID uint) (models.Folder, error) {
	if r.getByIDErr != nil {
		return models.Folder{}, r.getByIDErr
	}
	if folder, ok := r.foldersByID[folderID]; ok && folder.UserID == userID {
		return folder, nil
	}
	return r.fakeFolderRepo.GetByIDAndUser(ctx, tx, folderID, userID)
}

func (r *resolverFolderRepo) Create(ctx context.Context, tx *gorm.DB, folder *models.Folder) error {
	if r.createErr != nil {
		return r.createErr
	}
	return r.fakeFolderRepo.Create(ctx, tx, folder)
}

func TestFolderResolverGetOrCreateUserRootFolderReturnsExisting(t *testing.T) {
	repo := newResolverFolderRepo()
	isRoot := true
	repo.roots[1] = models.Folder{ID: 9, UserID: 1, Name: "root", Path: "/", IsRoot: &isRoot}

	resolver := folderResolver{folders: repo}
	root, err := resolver.getOrCreateUserRootFolder(context.Background(), nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root.ID != 9 {
		t.Fatalf("expected existing root id 9, got %d", root.ID)
	}
}

func TestFolderResolverGetOrCreateUserRootFolderCreatesWhenMissing(t *testing.T) {
	repo := newResolverFolderRepo()
	resolver := folderResolver{folders: repo}

	root, err := resolver.getOrCreateUserRootFolder(context.Background(), nil, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root.ID == 0 {
		t.Fatalf("expected new root id")
	}
	if root.UserID != 3 || root.Path != "/" || root.IsRoot == nil || !*root.IsRoot {
		t.Fatalf("unexpected root folder: %+v", root)
	}
}

func TestFolderResolverGetOrCreateUserRootFolderPropagatesErrors(t *testing.T) {
	repo := newResolverFolderRepo()
	repo.getRootErr = errors.New("db unavailable")

	resolver := folderResolver{folders: repo}
	_, err := resolver.getOrCreateUserRootFolder(context.Background(), nil, 1)
	if err == nil || err.Error() != "db unavailable" {
		t.Fatalf("expected root query error, got %v", err)
	}

	repo = newResolverFolderRepo()
	repo.createErr = errors.New("create failed")
	resolver = folderResolver{folders: repo}
	_, err = resolver.getOrCreateUserRootFolder(context.Background(), nil, 1)
	if err == nil || err.Error() != "create failed" {
		t.Fatalf("expected create error, got %v", err)
	}
}

func TestFolderResolverResolveFolderIDForUser(t *testing.T) {
	repo := newResolverFolderRepo()
	isRoot := true
	repo.roots[5] = models.Folder{ID: 12, UserID: 5, Name: "root", Path: "/", IsRoot: &isRoot}
	repo.foldersByID[33] = models.Folder{ID: 33, UserID: 5, Name: "docs", Path: "/docs"}

	resolver := folderResolver{folders: repo}

	resolved, err := resolver.resolveFolderIDForUser(context.Background(), nil, 5, 0)
	if err != nil {
		t.Fatalf("unexpected error for root resolution: %v", err)
	}
	if resolved != 12 {
		t.Fatalf("expected root folder id 12, got %d", resolved)
	}

	resolved, err = resolver.resolveFolderIDForUser(context.Background(), nil, 5, 33)
	if err != nil {
		t.Fatalf("unexpected error for folder resolution: %v", err)
	}
	if resolved != 33 {
		t.Fatalf("expected folder id 33, got %d", resolved)
	}

	repo.getByIDErr = gorm.ErrRecordNotFound
	_, err = resolver.resolveFolderIDForUser(context.Background(), nil, 5, 99)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record-not-found error, got %v", err)
	}
}

func TestBuildChildFolderPath(t *testing.T) {
	cases := []struct {
		parent string
		child  string
		want   string
	}{
		{parent: "", child: "docs", want: "/docs"},
		{parent: "/", child: "docs", want: "/docs"},
		{parent: "/parent/", child: "docs", want: "/parent/docs"},
		{parent: "/parent", child: "docs", want: "/parent/docs"},
	}

	for _, tc := range cases {
		if got := buildChildFolderPath(tc.parent, tc.child); got != tc.want {
			t.Fatalf("buildChildFolderPath(%q,%q) = %q, want %q", tc.parent, tc.child, got, tc.want)
		}
	}
}
