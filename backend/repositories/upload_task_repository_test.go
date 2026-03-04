package repositories

import (
	"context"
	"testing"
	"time"

	"mcloud/models"
)

func TestGormUploadTaskRepository_Create_BuildsInsertSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormUploadTaskRepository(db)

	task := &models.UploadTask{
		UploadID:    "u-1",
		UserID:      2,
		FileName:    "a.bin",
		FileSize:    100,
		FileMD5:     "abc",
		TotalChunks: 5,
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	if err := repo.Create(context.Background(), nil, task); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	assertLastSQLContains(t, rec, "insert into `upload_tasks`")
}

func TestGormUploadTaskRepository_GetByUploadID_BuildsLookupSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormUploadTaskRepository(db)

	_, err := repo.GetByUploadID(context.Background(), nil, "u-1")
	if err != nil {
		t.Fatalf("GetByUploadID failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `upload_tasks`", "where upload_id = ?")
}

func TestGormUploadTaskRepository_GetByUploadIDAndUser_BuildsLookupSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormUploadTaskRepository(db)

	_, err := repo.GetByUploadIDAndUser(context.Background(), nil, "u-1", 2)
	if err != nil {
		t.Fatalf("GetByUploadIDAndUser failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `upload_tasks`", "where upload_id = ? and user_id = ?")
}

func TestGormUploadTaskRepository_FindResumableBySignature_BuildsResumableSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormUploadTaskRepository(db)

	now := time.Now()
	_, err := repo.FindResumableBySignature(context.Background(), nil, 2, 3, "a.bin", 100, "abc", now)
	if err != nil {
		t.Fatalf("FindResumableBySignature failed: %v", err)
	}

	assertLastSQLContains(t, rec,
		"from `upload_tasks`",
		"user_id = ? and folder_id = ? and file_name = ? and file_size = ? and file_md5 = ?",
		"expires_at > ?",
		"status in",
		"order by updated_at desc",
	)
}

func TestGormUploadTaskRepository_ListVisibleByUser_BuildsVisibleFilterSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormUploadTaskRepository(db)

	_, err := repo.ListVisibleByUser(context.Background(), nil, 2, time.Now(), time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("ListVisibleByUser failed: %v", err)
	}

	assertLastSQLContains(t, rec,
		"from `upload_tasks`",
		"user_id = ?",
		"status != ? or completed_at >= ?",
		"order by updated_at desc",
	)
}

func TestGormUploadTaskRepository_UpdateStatus_BuildsUpdateSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormUploadTaskRepository(db)

	err := repo.UpdateStatus(context.Background(), nil, "u-1", "paused")
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	assertLastSQLContains(t, rec, "update `upload_tasks` set `status`", "where upload_id = ?")
}

func TestGormUploadTaskRepository_UpdateProgress_BuildsProgressUpdateSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormUploadTaskRepository(db)

	err := repo.UpdateProgress(context.Background(), nil, "u-1", 3, 60, time.Now())
	if err != nil {
		t.Fatalf("UpdateProgress failed: %v", err)
	}

	assertLastSQLContains(t, rec,
		"update `upload_tasks` set",
		"`uploaded_chunks_count`",
		"`uploaded_size`",
		"`last_chunk_at`",
		"`status`",
		"`last_error`",
		"where upload_id = ?",
	)
}

func TestGormUploadTaskRepository_MarkCompleted_BuildsCompletedUpdateSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormUploadTaskRepository(db)

	err := repo.MarkCompleted(context.Background(), nil, "u-1", time.Now())
	if err != nil {
		t.Fatalf("MarkCompleted failed: %v", err)
	}

	assertLastSQLContains(t, rec,
		"update `upload_tasks` set",
		"`status`",
		"`completed_at`",
		"`last_error`",
		"where upload_id = ?",
	)
}

func TestGormUploadTaskRepository_UpdateUploadedChunksSnapshot_BuildsUpdateSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormUploadTaskRepository(db)

	err := repo.UpdateUploadedChunksSnapshot(context.Background(), nil, "u-1", "[1,2,3]")
	if err != nil {
		t.Fatalf("UpdateUploadedChunksSnapshot failed: %v", err)
	}

	assertLastSQLContains(t, rec, "update `upload_tasks` set `uploaded_chunks`", "where upload_id = ?")
}

func TestGormUploadTaskRepository_DeleteByID_BuildsDeleteSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormUploadTaskRepository(db)

	err := repo.DeleteByID(context.Background(), nil, 7)
	if err != nil {
		t.Fatalf("DeleteByID failed: %v", err)
	}

	assertLastSQLContains(t, rec, "delete from `upload_tasks`", "where `upload_tasks`.`id` = ?")
}

func TestGormUploadTaskRepository_ListExpiredAndUncompleted_BuildsExpiredFilterSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormUploadTaskRepository(db)

	_, err := repo.ListExpiredAndUncompleted(context.Background(), nil, time.Now())
	if err != nil {
		t.Fatalf("ListExpiredAndUncompleted failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `upload_tasks`", "where expires_at < ? and status != ?")
}
