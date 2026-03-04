package repositories

import (
	"context"
	"testing"

	"mcloud/models"
)

func TestGormFileObjectRepository_Create_BuildsInsertSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileObjectRepository(db)

	obj := &models.FileObject{
		FilePath: "/tmp/a.txt",
		FileSize: 100,
		FileMD5:  "abc",
		RefCount: 1,
	}
	if err := repo.Create(context.Background(), nil, obj); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	assertLastSQLContains(t, rec, "insert into `file_objects`")
}

func TestGormFileObjectRepository_GetByID_BuildsPrimaryKeyLookupSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileObjectRepository(db)

	_, err := repo.GetByID(context.Background(), nil, 3)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `file_objects`", "where `file_objects`.`id` = ?")
}

func TestGormFileObjectRepository_GetByMD5_BuildsFilterSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileObjectRepository(db)

	_, err := repo.GetByMD5(context.Background(), nil, "abc")
	if err != nil {
		t.Fatalf("GetByMD5 failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `file_objects`", "where file_md5 = ?")
}

func TestGormFileObjectRepository_IncrementRefCount_BuildsUpdateSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileObjectRepository(db)

	err := repo.IncrementRefCount(context.Background(), nil, 5)
	if err != nil {
		t.Fatalf("IncrementRefCount failed: %v", err)
	}

	assertLastSQLContains(t, rec, "update `file_objects`", "`ref_count`=ref_count + 1", "where id = ?")
}

func TestGormFileObjectRepository_DecrementRefCount_BuildsUpdateSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileObjectRepository(db)

	err := repo.DecrementRefCount(context.Background(), nil, 5)
	if err != nil {
		t.Fatalf("DecrementRefCount failed: %v", err)
	}

	assertLastSQLContains(t, rec, "update `file_objects`", "`ref_count`=ref_count - 1", "where id = ?")
}

func TestGormFileObjectRepository_DeleteByID_BuildsDeleteSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormFileObjectRepository(db)

	err := repo.DeleteByID(context.Background(), nil, 5)
	if err != nil {
		t.Fatalf("DeleteByID failed: %v", err)
	}

	assertLastSQLContains(t, rec, "delete from `file_objects`", "where `file_objects`.`id` = ?")
}
