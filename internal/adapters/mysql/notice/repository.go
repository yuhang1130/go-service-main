package notice

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	noticeapp "github.com/yuhang1130/go-service-main/internal/features/notice/application"
	noticedomain "github.com/yuhang1130/go-service-main/internal/features/notice/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ database *gorm.DB }

func NewRepository(database *gorm.DB) *Repository { return &Repository{database: database} }

type noticeRow struct {
	ID            int64      `gorm:"column:id;primaryKey"`
	Title         string     `gorm:"column:title"`
	Content       string     `gorm:"column:content"`
	Type          int        `gorm:"column:type"`
	Level         string     `gorm:"column:level"`
	TargetType    int        `gorm:"column:target_type"`
	TargetUserIDs []byte     `gorm:"column:target_user_ids"`
	PublisherID   *int64     `gorm:"column:publisher_id"`
	PublisherName string     `gorm:"column:publisher_name;->"`
	PublishStatus int        `gorm:"column:publish_status"`
	PublishTime   *time.Time `gorm:"column:publish_time"`
	RevokeTime    *time.Time `gorm:"column:revoke_time"`
	IsRead        int        `gorm:"column:is_read;->"`
	CreateBy      int64      `gorm:"column:create_by"`
	CreateTime    time.Time  `gorm:"column:create_time"`
	UpdateBy      *int64     `gorm:"column:update_by"`
	UpdateTime    time.Time  `gorm:"column:update_time"`
	IsDeleted     int        `gorm:"column:is_deleted"`
}

func (noticeRow) TableName() string { return "sys_notice" }

func (r *Repository) List(ctx context.Context, query noticeapp.Query) ([]noticedomain.Notice, int64, error) {
	database := r.database.WithContext(ctx).Table("sys_notice n").Where("n.is_deleted = 0")
	database = applyQuery(database, query)
	var total int64
	if err := database.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []noticeRow
	if err := database.Select("n.*, COALESCE(u.nickname, '') AS publisher_name, 0 AS is_read").Joins("LEFT JOIN sys_user u ON u.id = n.publisher_id AND u.is_deleted = 0").Order("n.create_time DESC, n.id DESC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return toDomains(rows), total, nil
}

func (r *Repository) ListMine(ctx context.Context, userID int64, query noticeapp.Query) ([]noticedomain.Notice, int64, error) {
	database := r.visible(r.database.WithContext(ctx).Table("sys_notice n"), userID)
	database = applyQuery(database, query)
	if query.IsRead != nil {
		if *query.IsRead == 1 {
			database = database.Where("EXISTS (SELECT 1 FROM sys_user_notice un WHERE un.notice_id = n.id AND un.user_id = ? AND un.is_read = 1)", userID)
		} else {
			database = database.Where("NOT EXISTS (SELECT 1 FROM sys_user_notice un WHERE un.notice_id = n.id AND un.user_id = ? AND un.is_read = 1)", userID)
		}
	}
	var total int64
	if err := database.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []noticeRow
	selectSQL := "n.*, COALESCE(u.nickname, '') AS publisher_name, CASE WHEN un.is_read = 1 THEN 1 ELSE 0 END AS is_read"
	if err := database.Select(selectSQL).Joins("LEFT JOIN sys_user u ON u.id = n.publisher_id AND u.is_deleted = 0").Joins("LEFT JOIN sys_user_notice un ON un.notice_id = n.id AND un.user_id = ?", userID).Order("n.publish_time DESC, n.id DESC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return toDomains(rows), total, nil
}

func (r *Repository) Get(ctx context.Context, id int64) (noticedomain.Notice, error) {
	var row noticeRow
	err := r.database.WithContext(ctx).Table("sys_notice n").Select("n.*, COALESCE(u.nickname, '') AS publisher_name, 0 AS is_read").Joins("LEFT JOIN sys_user u ON u.id = n.publisher_id AND u.is_deleted = 0").Where("n.id = ? AND n.is_deleted = 0", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return noticedomain.Notice{}, noticeapp.ErrNotFound
	}
	return toDomain(row), err
}

func (r *Repository) GetVisible(ctx context.Context, id, userID int64) (noticedomain.Notice, error) {
	var row noticeRow
	database := r.visible(r.database.WithContext(ctx).Table("sys_notice n"), userID)
	err := database.Select("n.*, COALESCE(u.nickname, '') AS publisher_name, CASE WHEN un.is_read = 1 THEN 1 ELSE 0 END AS is_read").Joins("LEFT JOIN sys_user u ON u.id = n.publisher_id AND u.is_deleted = 0").Joins("LEFT JOIN sys_user_notice un ON un.notice_id = n.id AND un.user_id = ?", userID).Where("n.id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return noticedomain.Notice{}, noticeapp.ErrNotFound
	}
	return toDomain(row), err
}

func (r *Repository) Create(ctx context.Context, item noticedomain.Notice, actorID int64) (int64, error) {
	now := time.Now().UTC()
	row := noticeRow{Title: item.Title, Content: item.Content, Type: item.Type, Level: item.Level, TargetType: item.TargetType, TargetUserIDs: marshalIDs(item.TargetUserIDs), PublishStatus: item.PublishStatus, PublishTime: item.PublishTime, RevokeTime: item.RevokeTime, CreateBy: actorID, CreateTime: now, UpdateBy: &actorID, UpdateTime: now}
	if item.PublisherID > 0 {
		row.PublisherID = &item.PublisherID
	}
	if err := r.database.WithContext(ctx).Create(&row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

func (r *Repository) Update(ctx context.Context, item noticedomain.Notice, actorID int64) error {
	return r.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if _, err := lockExpectedState(transaction, item.ID, noticedomain.StatusDraft); err != nil {
			return err
		}
		updates := map[string]any{"title": item.Title, "content": item.Content, "type": item.Type, "level": item.Level, "target_type": item.TargetType, "target_user_ids": marshalIDs(item.TargetUserIDs), "publish_status": item.PublishStatus, "update_by": actorID, "update_time": time.Now().UTC()}
		if item.PublisherID > 0 {
			updates["publisher_id"] = item.PublisherID
			updates["publish_time"] = item.PublishTime
			updates["revoke_time"] = nil
		} else if item.PublishStatus == noticedomain.StatusDraft {
			updates["publisher_id"] = nil
			updates["publish_time"] = nil
			updates["revoke_time"] = nil
		} else if item.RevokeTime != nil {
			updates["revoke_time"] = item.RevokeTime
		}
		if err := transaction.Model(&noticeRow{}).Where("id = ? AND is_deleted = 0", item.ID).Updates(updates).Error; err != nil {
			return err
		}
		return transaction.Exec("DELETE FROM sys_user_notice WHERE notice_id = ?", item.ID).Error
	})
}

func (r *Repository) Delete(ctx context.Context, ids []int64, actorID int64) error {
	return r.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var rows []noticeRow
		if err := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "publish_status").Where("id IN ? AND is_deleted = 0", ids).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) != len(ids) {
			return noticeapp.ErrNotFound
		}
		for _, row := range rows {
			if row.PublishStatus == noticedomain.StatusPublished {
				return noticedomain.ErrInvalidTransition
			}
		}
		if err := transaction.Model(&noticeRow{}).Where("id IN ? AND is_deleted = 0", ids).Updates(map[string]any{"is_deleted": 1, "update_by": actorID, "update_time": time.Now().UTC()}).Error; err != nil {
			return err
		}
		return transaction.Exec("DELETE FROM sys_user_notice WHERE notice_id IN ?", ids).Error
	})
}

func (r *Repository) Publish(ctx context.Context, id, actorID int64, now time.Time) (noticedomain.Notice, error) {
	return r.transitionState(ctx, id, noticedomain.StatusDraft, noticedomain.StatusPublished, actorID, now)
}

func (r *Repository) Revoke(ctx context.Context, id, actorID int64, now time.Time) (noticedomain.Notice, error) {
	return r.transitionState(ctx, id, noticedomain.StatusPublished, noticedomain.StatusRevoked, actorID, now)
}

func (r *Repository) transitionState(ctx context.Context, id int64, expected, next int, actorID int64, now time.Time) (noticedomain.Notice, error) {
	var item noticedomain.Notice
	err := r.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		row, err := lockExpectedState(transaction, id, expected)
		if err != nil {
			return err
		}
		updates := map[string]any{"publish_status": next, "update_by": actorID, "update_time": now}
		if next == noticedomain.StatusPublished {
			updates["publisher_id"] = actorID
			updates["publish_time"] = now
			updates["revoke_time"] = nil
			row.PublisherID = &actorID
			row.PublishTime = &now
			row.RevokeTime = nil
		} else {
			updates["revoke_time"] = now
			row.RevokeTime = &now
		}
		result := transaction.Model(&noticeRow{}).Where("id = ? AND is_deleted = 0", id).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return noticeapp.ErrNotFound
		}
		if err := transaction.Exec("DELETE FROM sys_user_notice WHERE notice_id = ?", id).Error; err != nil {
			return err
		}
		row.PublishStatus = next
		row.UpdateBy = &actorID
		row.UpdateTime = now
		item = toDomain(row)
		return nil
	})
	return item, err
}

func lockExpectedState(transaction *gorm.DB, id int64, expected int) (noticeRow, error) {
	var row noticeRow
	err := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND is_deleted = 0", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return noticeRow{}, noticeapp.ErrNotFound
	}
	if err != nil {
		return noticeRow{}, err
	}
	if row.PublishStatus != expected {
		return noticeRow{}, noticedomain.ErrInvalidTransition
	}
	return row, nil
}

func (r *Repository) MarkRead(ctx context.Context, noticeID, userID int64, now time.Time) error {
	return r.database.WithContext(ctx).Exec(`INSERT INTO sys_user_notice (notice_id, user_id, is_read, read_time, create_time, update_time)
		VALUES (?, ?, 1, ?, ?, ?) ON DUPLICATE KEY UPDATE is_read = 1, read_time = VALUES(read_time), update_time = VALUES(update_time)`, noticeID, userID, now, now, now).Error
}

func (r *Repository) MarkAllRead(ctx context.Context, userID int64, now time.Time) error {
	return r.database.WithContext(ctx).Exec(`INSERT INTO sys_user_notice (notice_id, user_id, is_read, read_time, create_time, update_time)
		SELECT n.id, ?, 1, ?, ?, ? FROM sys_notice n
		WHERE n.is_deleted = 0 AND n.publish_status = 1
		AND (n.target_type = 1 OR (n.target_type = 2 AND JSON_CONTAINS(n.target_user_ids, JSON_ARRAY(?))))
		ON DUPLICATE KEY UPDATE is_read = 1, read_time = VALUES(read_time), update_time = VALUES(update_time)`, userID, now, now, now, userID).Error
}

func (r *Repository) UnreadCount(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.visible(r.database.WithContext(ctx).Table("sys_notice n"), userID).Where("NOT EXISTS (SELECT 1 FROM sys_user_notice un WHERE un.notice_id = n.id AND un.user_id = ? AND un.is_read = 1)", userID).Count(&count).Error
	return count, err
}

func (r *Repository) AccountsExist(ctx context.Context, ids []int64) (bool, error) {
	var count int64
	err := r.database.WithContext(ctx).Table("sys_user").Where("id IN ? AND is_deleted = 0", ids).Count(&count).Error
	return count == int64(len(ids)), err
}

func (r *Repository) visible(database *gorm.DB, userID int64) *gorm.DB {
	return database.Where("n.is_deleted = 0 AND n.publish_status = 1").Where("n.target_type = 1 OR (n.target_type = 2 AND JSON_CONTAINS(n.target_user_ids, JSON_ARRAY(?)))", userID)
}

func applyQuery(database *gorm.DB, query noticeapp.Query) *gorm.DB {
	if query.Title != "" {
		database = database.Where("n.title LIKE ?", "%"+query.Title+"%")
	}
	if query.Type != nil {
		database = database.Where("n.type = ?", *query.Type)
	}
	if query.Status != nil {
		database = database.Where("n.publish_status = ?", *query.Status)
	}
	return database
}

func marshalIDs(ids []int64) []byte {
	if ids == nil {
		ids = []int64{}
	}
	value, _ := json.Marshal(ids)
	return value
}

func toDomains(rows []noticeRow) []noticedomain.Notice {
	items := make([]noticedomain.Notice, len(rows))
	for index, row := range rows {
		items[index] = toDomain(row)
	}
	return items
}

func toDomain(row noticeRow) noticedomain.Notice {
	var targetUserIDs []int64
	_ = json.Unmarshal(row.TargetUserIDs, &targetUserIDs)
	publisherID := int64(0)
	if row.PublisherID != nil {
		publisherID = *row.PublisherID
	}
	return noticedomain.Notice{ID: row.ID, Title: row.Title, Content: row.Content, Type: row.Type, Level: row.Level, TargetType: row.TargetType, TargetUserIDs: targetUserIDs, PublisherID: publisherID, PublisherName: row.PublisherName, PublishStatus: row.PublishStatus, PublishTime: row.PublishTime, RevokeTime: row.RevokeTime, IsRead: row.IsRead, CreateTime: row.CreateTime, UpdateTime: row.UpdateTime}
}
