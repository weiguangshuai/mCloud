package repositories

import (
	"context"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"mcloud/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type sqlRecorder struct {
	gormlogger.Interface
	mu   sync.Mutex
	sqls []string
}

var (
	sqlStringLiteralPattern = regexp.MustCompile(`"[^"]*"|'[^']*'`)
	sqlNumericLiteralPattern = regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
)

func newSQLRecorder() *sqlRecorder {
	base := gormlogger.New(log.New(io.Discard, "", 0), gormlogger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  gormlogger.Info,
		IgnoreRecordNotFoundError: false,
		Colorful:                  false,
		ParameterizedQueries:      true,
	})
	return &sqlRecorder{Interface: base}
}

func (r *sqlRecorder) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	r.Interface = r.Interface.LogMode(level)
	return r
}

func (r *sqlRecorder) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	if strings.TrimSpace(sql) == "" {
		return
	}

	r.mu.Lock()
	r.sqls = append(r.sqls, sql)
	r.mu.Unlock()
}

func (r *sqlRecorder) Reset() {
	r.mu.Lock()
	r.sqls = nil
	r.mu.Unlock()
}

func (r *sqlRecorder) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.sqls)
}

func (r *sqlRecorder) Last() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.sqls) == 0 {
		return ""
	}
	return r.sqls[len(r.sqls)-1]
}

func newDryRunMySQL(t *testing.T) (*gorm.DB, *sqlRecorder) {
	t.Helper()

	rec := newSQLRecorder()
	dsn := fmt.Sprintf("file:repo_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:                 rec,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("open sqlite test db failed: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db failed: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	if err := db.AutoMigrate(
		&models.User{},
		&models.Folder{},
		&models.FileObject{},
		&models.File{},
		&models.UploadTask{},
		&models.RecycleBinItem{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	rec.Reset()
	return db.Session(&gorm.Session{DryRun: true}), rec
}

func assertLastSQLContains(t *testing.T, rec *sqlRecorder, fragments ...string) {
	t.Helper()

	sql := normalizeSQL(rec.Last())
	if sql == "" {
		t.Fatalf("expected SQL to be captured, got none")
	}
	for _, fragment := range fragments {
		if !strings.Contains(sql, normalizeSQL(fragment)) {
			t.Fatalf("expected SQL to contain %q, got %q", fragment, rec.Last())
		}
	}
}

func assertNoSQLCaptured(t *testing.T, rec *sqlRecorder) {
	t.Helper()
	if rec.Count() != 0 {
		t.Fatalf("expected no SQL to be captured, got %d (%s)", rec.Count(), rec.Last())
	}
}

func assertLastSQLNotContains(t *testing.T, rec *sqlRecorder, fragments ...string) {
	t.Helper()

	sql := normalizeSQL(rec.Last())
	if sql == "" {
		t.Fatalf("expected SQL to be captured, got none")
	}
	for _, fragment := range fragments {
		if strings.Contains(sql, normalizeSQL(fragment)) {
			t.Fatalf("expected SQL not to contain %q, got %q", fragment, rec.Last())
		}
	}
}

func normalizeSQL(sql string) string {
	sql = strings.ToLower(sql)
	sql = strings.NewReplacer(
		"`", "",
		"(", " ",
		")", " ",
		"\n", " ",
		"\t", " ",
	).Replace(sql)
	sql = sqlStringLiteralPattern.ReplaceAllString(sql, "?")
	sql = sqlNumericLiteralPattern.ReplaceAllString(sql, "?")
	return strings.Join(strings.Fields(sql), " ")
}
