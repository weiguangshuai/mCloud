package repositories

import (
	"context"
	"testing"
	"time"

	"mcloud/models"
)

func TestGormRecycleBinRepository_CountByUser_BuildsCountSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormRecycleBinRepository(db)

	total, err := repo.CountByUser(context.Background(), nil, 2)
	if err != nil {
		t.Fatalf("CountByUser failed: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected dry-run total 0, got %d", total)
	}

	assertLastSQLContains(t, rec, "select count(*)", "from `recycle_bin`", "where user_id = ?")
}

func TestGormRecycleBinRepository_ListByUser_WithSortSQL_BuildsOrderedQuery(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormRecycleBinRepository(db)

	_, err := repo.ListByUser(context.Background(), nil, RecycleBinListInput{
		UserID:  2,
		Offset:  10,
		Limit:   20,
		SortSQL: "expires_at DESC",
	})
	if err != nil {
		t.Fatalf("ListByUser failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `recycle_bin`", "where user_id = ?", "order by expires_at desc")
}

func TestGormRecycleBinRepository_ListByUser_WithoutSortSQL_OmitsOrderClause(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormRecycleBinRepository(db)

	_, err := repo.ListByUser(context.Background(), nil, RecycleBinListInput{
		UserID: 2,
		Offset: 0,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("ListByUser failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `recycle_bin`", "where user_id = ?")
	assertLastSQLNotContains(t, rec, "order by")
}

func TestGormRecycleBinRepository_ListAllByUser_BuildsSelectSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormRecycleBinRepository(db)

	_, err := repo.ListAllByUser(context.Background(), nil, 2)
	if err != nil {
		t.Fatalf("ListAllByUser failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `recycle_bin`", "where user_id = ?")
}

func TestGormRecycleBinRepository_ListExpired_BuildsExpireFilterSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormRecycleBinRepository(db)

	_, err := repo.ListExpired(context.Background(), nil, time.Now())
	if err != nil {
		t.Fatalf("ListExpired failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `recycle_bin`", "where expires_at < ?")
}

func TestGormRecycleBinRepository_GetByIDAndUser_BuildsLookupSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormRecycleBinRepository(db)

	_, err := repo.GetByIDAndUser(context.Background(), nil, 7, 2)
	if err != nil {
		t.Fatalf("GetByIDAndUser failed: %v", err)
	}

	assertLastSQLContains(t, rec, "from `recycle_bin`", "where id = ? and user_id = ?")
}

func TestGormRecycleBinRepository_Create_BuildsInsertSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormRecycleBinRepository(db)

	item := &models.RecycleBinItem{
		UserID:       2,
		OriginalID:   8,
		OriginalType: "file",
		OriginalName: "a.txt",
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	if err := repo.Create(context.Background(), nil, item); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	assertLastSQLContains(t, rec, "insert into `recycle_bin`")
}

func TestGormRecycleBinRepository_DeleteByID_BuildsDeleteSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormRecycleBinRepository(db)

	err := repo.DeleteByID(context.Background(), nil, 7)
	if err != nil {
		t.Fatalf("DeleteByID failed: %v", err)
	}

	assertLastSQLContains(t, rec, "delete from `recycle_bin`", "where `recycle_bin`.`id` = ?")
}

func TestGormRecycleBinRepository_DeleteByUser_BuildsDeleteSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormRecycleBinRepository(db)

	err := repo.DeleteByUser(context.Background(), nil, 2)
	if err != nil {
		t.Fatalf("DeleteByUser failed: %v", err)
	}

	assertLastSQLContains(t, rec, "delete from `recycle_bin`", "where user_id = ?")
}

func TestGormRecycleBinRepository_DeleteByOriginalIDs_EmptyIDs_NoSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormRecycleBinRepository(db)

	err := repo.DeleteByOriginalIDs(context.Background(), nil, 2, "file", nil)
	if err != nil {
		t.Fatalf("DeleteByOriginalIDs failed: %v", err)
	}

	assertNoSQLCaptured(t, rec)
}

func TestGormRecycleBinRepository_DeleteByOriginalIDs_BuildsBatchDeleteSQL(t *testing.T) {
	db, rec := newDryRunMySQL(t)
	repo := NewGormRecycleBinRepository(db)

	err := repo.DeleteByOriginalIDs(context.Background(), nil, 2, "file", []uint{1, 2, 3})
	if err != nil {
		t.Fatalf("DeleteByOriginalIDs failed: %v", err)
	}

	assertLastSQLContains(t, rec, "delete from `recycle_bin`", "where user_id = ? and original_type = ? and original_id in")
}
