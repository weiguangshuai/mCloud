package repositories

import (
	"context"
	"testing"

	"mcloud/models"
)

func TestGormFileRepository_CountByFolder_NormalFolder_BuildsScopedSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileRepository(db)

	total, err := repo.CountByFolder(context.Background(), nil, 2, 9, 1, false)
	if err != nil {
		t.Fatalf("CountByFolder failed: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected dry-run total 0, got %d", total)
	}

	assertLastSQLContains(t, rec, "select count(*)", "from `files`", "where user_id = ? and folder_id = ?")
}

func TestGormFileRepository_CountByFolder_LegacyRoot_BuildsCompatSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileRepository(db)

	_, err := repo.CountByFolder(context.Background(), nil, 2, 1, 1, true)
	if err != nil {
		t.Fatalf("CountByFolder with legacy root failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `files`", "where user_id = ? and (folder_id = ? or folder_id = 0)")
}

func TestGormFileRepository_CountByFolderAndOriginalName_ScopedWithoutExclude(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileRepository(db)

	_, err := repo.CountByFolderAndOriginalName(context.Background(), nil, 2, 9, "a.txt", 0, false)
	if err != nil {
		t.Fatalf("CountByFolderAndOriginalName failed: %v", err)
	}

	assertLastSQLContains(t, rec, "select count(*)", "from `files`", "user_id = ? and folder_id = ? and original_name = ?")
	assertLastSQLNotContains(t, rec, "id <> ?")
}

func TestGormFileRepository_CountByFolderAndOriginalName_UnscopedWithExclude(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileRepository(db)

	_, err := repo.CountByFolderAndOriginalName(context.Background(), nil, 2, 9, "a.txt", 88, true)
	if err != nil {
		t.Fatalf("CountByFolderAndOriginalName failed: %v", err)
	}

	assertLastSQLContains(t, rec, "select count(*)", "from `files`", "id <> ?")
	assertLastSQLNotContains(t, rec, "deleted_at is null")
}

func TestGormFileRepository_ListByFolder_DefaultSort_FallsBackToCreatedAtDesc(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileRepository(db)

	_, err := repo.ListByFolder(context.Background(), nil, ListFilesInput{
		UserID:            2,
		FolderID:          9,
		RootFolderID:      1,
		IncludeLegacyRoot: false,
		SortBy:            "invalid_sort",
		Order:             "invalid",
		Offset:            0,
		Limit:             20,
	})
	if err != nil {
		t.Fatalf("ListByFolder failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `files`", "where user_id = ? and folder_id = ?", "order by files.created_at desc")
}

func TestGormFileRepository_ListByFolder_SortByFileSize_UsesJoinAndOrder(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileRepository(db)

	_, err := repo.ListByFolder(context.Background(), nil, ListFilesInput{
		UserID:            2,
		FolderID:          1,
		RootFolderID:      1,
		IncludeLegacyRoot: true,
		SortBy:            "file_size",
		Order:             "asc",
		Offset:            5,
		Limit:             10,
	})
	if err != nil {
		t.Fatalf("ListByFolder by file_size failed: %v", err)
	}

	assertLastSQLContains(t, rec,
		"from `files`",
		"left join file_objects on file_objects.id = files.file_object_id",
		"where user_id = ? and (folder_id = ? or folder_id = 0)",
		"order by file_objects.file_size asc",
	)
}

func TestGormFileRepository_ListByFolderIDs_BuildsINQuery(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileRepository(db)

	_, err := repo.ListByFolderIDs(context.Background(), nil, 3, []uint{1, 2}, false, true)
	if err != nil {
		t.Fatalf("ListByFolderIDs failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `files`", "where user_id = ? and folder_id in")
	assertLastSQLNotContains(t, rec, "deleted_at is null")
}

func TestGormFileRepository_Create_BuildsInsertSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileRepository(db)

	file := &models.File{
		Name:         "a.txt",
		OriginalName: "a.txt",
		FolderID:     1,
		UserID:       2,
		FileObjectID: 3,
	}
	if err := repo.Create(context.Background(), nil, file); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	assertLastSQLContains(t, rec, "insert into `files`")
}

func TestGormFileRepository_GetByIDAndUser_BuildsSelectSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileRepository(db)

	_, err := repo.GetByIDAndUser(context.Background(), nil, 7, 2, false)
	if err != nil {
		t.Fatalf("GetByIDAndUser failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `files`", "where id = ? and user_id = ?")
}

func TestGormFileRepository_GetByIDAndUserUnscoped_BuildsUnscopedSelectSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileRepository(db)

	_, err := repo.GetByIDAndUserUnscoped(context.Background(), nil, 7, 2, false)
	if err != nil {
		t.Fatalf("GetByIDAndUserUnscoped failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `files`", "where id = ? and user_id = ?")
	assertLastSQLNotContains(t, rec, "deleted_at is null")
}

func TestGormFileRepository_GetByIDsAndUser_BuildsINQuery(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileRepository(db)

	_, err := repo.GetByIDsAndUser(context.Background(), nil, 2, []uint{7, 8}, false)
	if err != nil {
		t.Fatalf("GetByIDsAndUser failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `files`", "where user_id = ? and id in")
}

func TestGormFileRepository_UpdateByIDAndUser_BuildsUpdateSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileRepository(db)

	err := repo.UpdateByIDAndUser(context.Background(), nil, 7, 2, map[string]interface{}{"name": "b.txt"})
	if err != nil {
		t.Fatalf("UpdateByIDAndUser failed: %v", err)
	}

	assertLastSQLContains(t, rec, "update `files` set", "where id = ? and user_id = ?")
}

func TestGormFileRepository_UpdateByIDsAndUser_EmptyIDs_NoSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileRepository(db)

	err := repo.UpdateByIDsAndUser(context.Background(), nil, nil, 2, map[string]interface{}{"name": "b.txt"})
	if err != nil {
		t.Fatalf("UpdateByIDsAndUser failed: %v", err)
	}

	assertNoSQLCaptured(t, rec)
}

func TestGormFileRepository_UpdateByIDsAndUser_BuildsBatchUpdateSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileRepository(db)

	err := repo.UpdateByIDsAndUser(context.Background(), nil, []uint{1, 2}, 2, map[string]interface{}{"name": "b.txt"})
	if err != nil {
		t.Fatalf("UpdateByIDsAndUser failed: %v", err)
	}

	assertLastSQLContains(t, rec, "update `files` set", "where id in", "and user_id = ?")
}

func TestGormFileRepository_SoftDeleteByIDAndUser_BuildsSoftDeleteSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileRepository(db)

	err := repo.SoftDeleteByIDAndUser(context.Background(), nil, 7, 2)
	if err != nil {
		t.Fatalf("SoftDeleteByIDAndUser failed: %v", err)
	}

	assertLastSQLContains(t, rec, "update `files` set `deleted_at`", "where (id = ? and user_id = ?)")
}

func TestGormFileRepository_SoftDeleteByFolderIDs_EmptyIDs_NoSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileRepository(db)

	err := repo.SoftDeleteByFolderIDs(context.Background(), nil, 2, nil)
	if err != nil {
		t.Fatalf("SoftDeleteByFolderIDs failed: %v", err)
	}

	assertNoSQLCaptured(t, rec)
}

func TestGormFileRepository_SoftDeleteByFolderIDs_BuildsBatchSoftDeleteSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileRepository(db)

	err := repo.SoftDeleteByFolderIDs(context.Background(), nil, 2, []uint{1, 2})
	if err != nil {
		t.Fatalf("SoftDeleteByFolderIDs failed: %v", err)
	}

	assertLastSQLContains(t, rec, "update `files` set `deleted_at`", "where (user_id = ? and folder_id in")
}

func TestGormFileRepository_UnscopedDeleteByIDAndUser_BuildsHardDeleteSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileRepository(db)

	err := repo.UnscopedDeleteByIDAndUser(context.Background(), nil, 7, 2)
	if err != nil {
		t.Fatalf("UnscopedDeleteByIDAndUser failed: %v", err)
	}

	assertLastSQLContains(t, rec, "delete from `files`", "where id = ? and user_id = ?")
}

func TestGormFileRepository_UnscopedRestoreByIDAndUser_BuildsUnscopedUpdateSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileRepository(db)

	err := repo.UnscopedRestoreByIDAndUser(context.Background(), nil, 7, 2, map[string]interface{}{"deleted_at": nil, "folder_id": 1})
	if err != nil {
		t.Fatalf("UnscopedRestoreByIDAndUser failed: %v", err)
	}

	assertLastSQLContains(t, rec, "update `files` set", "where id = ? and user_id = ?")
	assertLastSQLNotContains(t, rec, "deleted_at is null")
}

func TestGormFileRepository_UnscopedRestoreByFolderIDs_EmptyIDs_NoSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileRepository(db)

	err := repo.UnscopedRestoreByFolderIDs(context.Background(), nil, 2, nil, map[string]interface{}{"deleted_at": nil})
	if err != nil {
		t.Fatalf("UnscopedRestoreByFolderIDs failed: %v", err)
	}

	assertNoSQLCaptured(t, rec)
}

func TestGormFileRepository_UnscopedRestoreByFolderIDs_BuildsBatchUpdateSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileRepository(db)

	err := repo.UnscopedRestoreByFolderIDs(context.Background(), nil, 2, []uint{1, 2}, map[string]interface{}{"deleted_at": nil})
	if err != nil {
		t.Fatalf("UnscopedRestoreByFolderIDs failed: %v", err)
	}

	assertLastSQLContains(t, rec, "update `files` set", "where user_id = ? and folder_id in")
	assertLastSQLNotContains(t, rec, "deleted_at is null")
}

func TestGormFileRepository_FindByUserAndMD5_BuildsJoinSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileRepository(db)

	_, err := repo.FindByUserAndMD5(context.Background(), nil, 2, "abc")
	if err != nil {
		t.Fatalf("FindByUserAndMD5 failed: %v", err)
	}

	assertLastSQLContains(t, rec,
		"from `file_objects`",
		"join files on files.file_object_id = file_objects.id",
		"files.user_id = ? and file_objects.file_md5 = ? and files.deleted_at is null",
	)
}
