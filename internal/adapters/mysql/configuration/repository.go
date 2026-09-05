package configuration

import (
	"context"
	"errors"
	"strings"
	"time"

	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	configurationapp "github.com/yuhang1130/go-service-main/internal/features/configuration/application"
	configurationdomain "github.com/yuhang1130/go-service-main/internal/features/configuration/domain"
	"gorm.io/gorm"
)

type Repository struct{ database *gorm.DB }

func NewRepository(database *gorm.DB) *Repository { return &Repository{database: database} }

type configRow struct {
	ID          int64     `gorm:"column:id;primaryKey"`
	ConfigName  string    `gorm:"column:config_name"`
	ConfigKey   string    `gorm:"column:config_key"`
	ConfigValue string    `gorm:"column:config_value"`
	Remark      string    `gorm:"column:remark"`
	CreateBy    *int64    `gorm:"column:create_by"`
	CreateTime  time.Time `gorm:"column:create_time"`
	UpdateBy    *int64    `gorm:"column:update_by"`
	UpdateTime  time.Time `gorm:"column:update_time"`
	IsDeleted   int       `gorm:"column:is_deleted"`
}

func (configRow) TableName() string { return "sys_config" }

func (r *Repository) List(ctx context.Context, query configurationapp.Query) ([]configurationdomain.Config, int64, error) {
	database := r.database.WithContext(ctx).Model(&configRow{}).Where("is_deleted = 0")
	if query.Keywords != "" {
		keyword := "%" + query.Keywords + "%"
		database = database.Where("config_name LIKE ? OR config_key LIKE ?", keyword, keyword)
	}
	var total int64
	if err := database.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []configRow
	if err := database.Order("id ASC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	items := make([]configurationdomain.Config, len(rows))
	for index, row := range rows {
		items[index] = toDomain(row)
	}
	return items, total, nil
}

func (r *Repository) Get(ctx context.Context, id int64) (configurationdomain.Config, error) {
	var row configRow
	err := r.database.WithContext(ctx).Where("id = ? AND is_deleted = 0", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return configurationdomain.Config{}, configurationapp.ErrNotFound
	}
	return toDomain(row), err
}

func (r *Repository) GetByKey(ctx context.Context, key string) (configurationdomain.Config, error) {
	var row configRow
	err := r.database.WithContext(ctx).Where("config_key = ? AND is_deleted = 0", key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return configurationdomain.Config{}, configurationapp.ErrNotFound
	}
	return toDomain(row), err
}

func (r *Repository) KeyExists(ctx context.Context, key string, excludeID int64) (bool, error) {
	var count int64
	err := r.database.WithContext(ctx).Model(&configRow{}).Where("config_key = ? AND id <> ? AND is_deleted = 0", key, excludeID).Count(&count).Error
	return count > 0, err
}

func (r *Repository) Create(ctx context.Context, item configurationdomain.Config, actorID int64) error {
	now := time.Now().UTC()
	row := configRow{ConfigName: item.Name, ConfigKey: item.Key, ConfigValue: item.Value, Remark: item.Remark, CreateBy: &actorID, UpdateBy: &actorID, CreateTime: now, UpdateTime: now}
	return mysqladapter.NormalizeError(r.database.WithContext(ctx).Create(&row).Error)
}

func (r *Repository) Update(ctx context.Context, item configurationdomain.Config, actorID int64) error {
	result := r.database.WithContext(ctx).Model(&configRow{}).Where("id = ? AND is_deleted = 0", item.ID).Updates(map[string]any{
		"config_name": item.Name, "config_key": item.Key, "config_value": item.Value, "remark": item.Remark,
		"update_by": actorID, "update_time": time.Now().UTC(),
	})
	if result.Error != nil {
		return mysqladapter.NormalizeError(result.Error)
	}
	if result.RowsAffected != 1 {
		return configurationapp.ErrNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id, actorID int64) error {
	result := r.database.WithContext(ctx).Model(&configRow{}).Where("id = ? AND is_deleted = 0", id).Updates(map[string]any{
		"is_deleted": 1, "update_by": actorID, "update_time": time.Now().UTC(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return configurationapp.ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteMany(ctx context.Context, ids []int64, actorID int64) error {
	return r.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var count int64
		if err := transaction.Model(&configRow{}).Where("id IN ? AND is_deleted = 0", ids).Count(&count).Error; err != nil {
			return err
		}
		if count != int64(len(ids)) {
			return configurationapp.ErrNotFound
		}
		result := transaction.Model(&configRow{}).Where("id IN ? AND is_deleted = 0", ids).Updates(map[string]any{
			"is_deleted": 1, "update_by": actorID, "update_time": time.Now().UTC(),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != int64(len(ids)) {
			return configurationapp.ErrNotFound
		}
		return nil
	})
}

func toDomain(row configRow) configurationdomain.Config {
	return configurationdomain.Config{ID: row.ID, Name: strings.TrimSpace(row.ConfigName), Key: row.ConfigKey, Value: row.ConfigValue, Remark: row.Remark, CreateTime: row.CreateTime, UpdateTime: row.UpdateTime}
}
