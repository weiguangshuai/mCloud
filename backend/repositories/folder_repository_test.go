package repositories

import (
	"context"
	"testing"

	"mcloud/models"
)

func TestGormFolderRepository_GetByIDAndUser_BuildsSelectSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFolderRepository(db)

	_, err := repo.GetByIDAndUser(context.Background(), nil, 10, 2)
	if err != nil {
		t.Fatalf("GetByIDAndUser failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `folders`", "where id = ? and user_id = ?")
}

func TestGormFolderRepository_GetByIDAndUserUnscoped_BuildsUnscopedSelectSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFolderRepository(db)

	_, err := repo.GetByIDAndUserUnscoped(context.Background(), nil, 10, 2)
	if err != nil {
		t.Fatalf("GetByIDAndUserUnscoped failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `folders`", "where id = ? and user_id = ?")
	assertLastSQLNotContains(t, rec, "deleted_at is null")
}

func TestGormFolderRepository_GetRootByUser_BuildsRootLookupSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFolderRepository(db)

	_, err := repo.GetRootByUser(context.Background(), nil, 9)
	if err != nil {
		t.Fatalf("GetRootByUser failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `folders`", "user_id = ?", "is_root = 1")
}

func TestGormFolderRepository_Create_BuildsInsertSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFolderRepository(db)

	folder := &models.Folder{
		Name:   "docs",
		UserID: 2,
		Path:   "/docs",
	}
	if err := repo.Create(context.Background(), nil, folder); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	assertLastSQLContains(t, rec, "insert into `folders`")
}

func TestGormFolderRepository_ListByParent_NormalParent_BuildsScopedSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFolderRepository(db)

	_, err := repo.ListByParent(context.Background(), nil, 3, 8, false)
	if err != nil {
		t.Fatalf("ListByParent failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `folders`", "where user_id = ? and parent_id = ?", "order by name asc")
	assertLastSQLNotContains(t, rec, "parent_id is null")
}

func TestGormFolderRepository_ListByParent_IncludeLegacyRoot_BuildsCompatSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFolderRepository(db)

	_, err := repo.ListByParent(context.Background(), nil, 3, 8, true)
	if err != nil {
		t.Fatalf("ListByParent with legacy root failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `folders`", "where user_id = ?", "parent_id is null", "is_root is null", "order by name asc")
}

func TestGormFolderRepository_CountByParentAndName_WithExclude_BuildsExcludeSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFolderRepository(db)

	count, err := repo.CountByParentAndName(context.Background(), nil, 2, 5, "docs", 99)
	if err != nil {
		t.Fatalf("CountByParentAndName failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected dry-run count 0, got %d", count)
	}

	assertLastSQLContains(t, rec, "select count(*)", "from `folders`", "user_id = ?", "parent_id = ?", "name = ?", "id <> ?")
}

func TestGormFolderRepository_CountByParentAndName_WithoutExclude_OmitsExcludeCondition(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFolderRepository(db)

	_, err := repo.CountByParentAndName(context.Background(), nil, 2, 5, "docs", 0)
	if err != nil {
		t.Fatalf("CountByParentAndName failed: %v", err)
	}

	assertLastSQLContains(t, rec, "select count(*)", "from `folders`", "name = ?")
	assertLastSQLNotContains(t, rec, "id <> ?")
}

func TestGormFolderRepository_UpdateByID_BuildsUpdateSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFolderRepository(db)

	err := repo.UpdateByID(context.Background(), nil, 6, map[string]interface{}{"name": "new-name"})
	if err != nil {
		t.Fatalf("UpdateByID failed: %v", err)
	}

	assertLastSQLContains(t, rec, "update `folders` set", "where id = ?")
}

func TestGormFolderRepository_UpdateByIDUnscoped_BuildsUnscopedUpdateSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFolderRepository(db)

	err := repo.UpdateByIDUnscoped(context.Background(), nil, 6, map[string]interface{}{"name": "new-name"})
	if err != nil {
		t.Fatalf("UpdateByIDUnscoped failed: %v", err)
	}

	assertLastSQLContains(t, rec, "update `folders` set", "where id = ?")
	assertLastSQLNotContains(t, rec, "deleted_at is null")
}

func TestGormFolderRepository_ListByPathPrefix_Scoped_BuildsPathSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFolderRepository(db)

	_, err := repo.ListByPathPrefix(context.Background(), nil, 2, 8, "/a/b", false)
	if err != nil {
		t.Fatalf("ListByPathPrefix failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `folders`", "user_id = ?", "id = ? or path like ?")
}

func TestGormFolderRepository_ListByPathPrefix_Unscoped_OmitsSoftDeleteFilter(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFolderRepository(db)

	_, err := repo.ListByPathPrefix(context.Background(), nil, 2, 8, "/a/b", true)
	if err != nil {
		t.Fatalf("ListByPathPrefix unscoped failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `folders`", "id = ? or path like ?")
	assertLastSQLNotContains(t, rec, "deleted_at is null")
}

func TestGormFolderRepository_PluckIDsByPathPrefix_BuildsPluckSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFolderRepository(db)

	ids, err := repo.PluckIDsByPathPrefix(context.Background(), nil, 2, 8, "/a/b")
	if err != nil {
		t.Fatalf("PluckIDsByPathPrefix failed: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected dry-run empty ids, got %v", ids)
	}

	assertLastSQLContains(t, rec, "select `id` from `folders`", "user_id = ?", "id = ? or path like ?")
}

func TestGormFolderRepository_SoftDeleteByPathPrefix_BuildsSoftDeleteSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFolderRepository(db)

	err := repo.SoftDeleteByPathPrefix(context.Background(), nil, 2, 8, "/a/b")
	if err != nil {
		t.Fatalf("SoftDeleteByPathPrefix failed: %v", err)
	}

	assertLastSQLContains(t, rec, "update `folders` set `deleted_at`", "where (user_id = ? and (id = ? or path like ?))")
}

func TestGormFolderRepository_UnscopedDeleteByIDs_Empty_NoSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFolderRepository(db)

	err := repo.UnscopedDeleteByIDs(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("UnscopedDeleteByIDs failed: %v", err)
	}

	assertNoSQLCaptured(t, rec)
}

func TestGormFolderRepository_UnscopedDeleteByIDs_BuildsDeleteSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFolderRepository(db)

	err := repo.UnscopedDeleteByIDs(context.Background(), nil, []uint{1, 2, 3})
	if err != nil {
		t.Fatalf("UnscopedDeleteByIDs failed: %v", err)
	}

	assertLastSQLContains(t, rec, "delete from `folders`", "where id in")
}
