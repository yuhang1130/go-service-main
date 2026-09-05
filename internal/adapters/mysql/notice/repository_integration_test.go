//go:build integration

package notice

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	noticeapp "github.com/yuhang1130/go-service-main/internal/features/notice/application"
	noticedomain "github.com/yuhang1130/go-service-main/internal/features/notice/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
	"gorm.io/gorm"
)

func TestSpecifiedNoticeVisibilityAndReadReceipt(t *testing.T) {
	dsn := os.Getenv("APP_MYSQL_DSN")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("APP_MYSQL_DSN is required in CI")
		}
		t.Skip("APP_MYSQL_DSN is not set")
	}
	ctx := context.Background()
	cfg := config.Defaults().MySQL
	cfg.DSN = dsn
	database, err := mysqladapter.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	suffix := uuid.NewString()
	targetID := insertTestAccount(t, database.GORM(), "notice_target_"+suffix)
	otherID := insertTestAccount(t, database.GORM(), "notice_other_"+suffix)
	repository := NewRepository(database.GORM())
	notice := noticedomain.Notice{Title: "integration " + suffix, Content: "<p>content</p>", Type: 1, Level: "H", TargetType: noticedomain.TargetSpecified, TargetUserIDs: []int64{targetID}, PublisherID: otherID, PublishStatus: noticedomain.StatusPublished}
	now := time.Now().UTC()
	notice.PublishTime = &now
	if _, err := repository.Create(ctx, notice, otherID); err != nil {
		t.Fatal(err)
	}
	var noticeID int64
	if err := database.GORM().Table("sys_notice").Where("title = ?", notice.Title).Pluck("id", &noticeID).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		database.GORM().Exec("DELETE FROM sys_user_notice WHERE notice_id = ?", noticeID)
		database.GORM().Exec("DELETE FROM sys_notice WHERE id = ?", noticeID)
		database.GORM().Exec("DELETE FROM sys_user WHERE id IN ?", []int64{targetID, otherID})
	})

	if _, err := repository.GetVisible(ctx, noticeID, targetID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.GetVisible(ctx, noticeID, otherID); !errors.Is(err, noticeapp.ErrNotFound) {
		t.Fatalf("non-target visibility error = %v", err)
	}
	if count, err := repository.UnreadCount(ctx, targetID); err != nil || count < 1 {
		t.Fatalf("unread count = %d, err = %v", count, err)
	}
	if err := repository.MarkRead(ctx, noticeID, targetID, now); err != nil {
		t.Fatal(err)
	}
	item, err := repository.GetVisible(ctx, noticeID, targetID)
	if err != nil || item.IsRead != 1 {
		t.Fatalf("read state = %d, err = %v", item.IsRead, err)
	}
}

func TestRepositoryEnforcesNoticeStateTransitions(t *testing.T) {
	dsn := os.Getenv("APP_MYSQL_DSN")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("APP_MYSQL_DSN is required in CI")
		}
		t.Skip("APP_MYSQL_DSN is not set")
	}
	ctx := context.Background()
	cfg := config.Defaults().MySQL
	cfg.DSN = dsn
	database, err := mysqladapter.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	repository := NewRepository(database.GORM())
	noticeID, err := repository.Create(ctx, noticedomain.Notice{Title: "state " + uuid.NewString(), Content: "content", Type: 1, Level: "L", TargetType: noticedomain.TargetAll, PublishStatus: noticedomain.StatusDraft}, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.GORM().Exec("DELETE FROM sys_notice WHERE id = ?", noticeID) })
	now := time.Now().UTC()
	if _, err := repository.Publish(ctx, noticeID, 1, now); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Publish(ctx, noticeID, 1, now); !errors.Is(err, noticedomain.ErrInvalidTransition) {
		t.Fatalf("second publish error = %v, want invalid transition", err)
	}
	if _, err := repository.Revoke(ctx, noticeID, 1, now); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Revoke(ctx, noticeID, 1, now); !errors.Is(err, noticedomain.ErrInvalidTransition) {
		t.Fatalf("second revoke error = %v, want invalid transition", err)
	}
}

func insertTestAccount(t *testing.T, database *gorm.DB, username string) int64 {
	t.Helper()
	now := time.Now().UTC()
	if err := database.Exec("INSERT INTO sys_user (username, nickname, gender, password, status, create_time, update_time, is_deleted) VALUES (?, ?, 0, ?, 1, ?, ?, 0)", username, username, "test-only-hash", now, now).Error; err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := database.Table("sys_user").Where("username = ?", username).Pluck("id", &id).Error; err != nil {
		t.Fatal(err)
	}
	return id
}
