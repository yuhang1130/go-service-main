package organization

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	"github.com/yuhang1130/go-service-main/internal/features/organization/application"
	"github.com/yuhang1130/go-service-main/internal/features/organization/domain"
	"gorm.io/gorm"
)

type Repository struct{ database *gorm.DB }

func NewRepository(database *gorm.DB) *Repository { return &Repository{database: database} }

type departmentRow struct {
	ID         int64     `gorm:"column:id;primaryKey"`
	Name       string    `gorm:"column:name"`
	Code       string    `gorm:"column:code"`
	ParentID   int64     `gorm:"column:parent_id"`
	TreePath   string    `gorm:"column:tree_path"`
	Sort       int       `gorm:"column:sort"`
	Status     int       `gorm:"column:status"`
	CreateBy   *int64    `gorm:"column:create_by"`
	CreateTime time.Time `gorm:"column:create_time"`
	UpdateBy   *int64    `gorm:"column:update_by"`
	UpdateTime time.Time `gorm:"column:update_time"`
	IsDeleted  int       `gorm:"column:is_deleted"`
}

func (departmentRow) TableName() string { return "sys_dept" }

func (r *Repository) List(ctx context.Context, query application.Query) ([]domain.Department, error) {
	database := r.database.WithContext(ctx).Model(&departmentRow{}).Where("is_deleted = ?", 0)
	if keyword := strings.TrimSpace(query.Keywords); keyword != "" {
		database = database.Where("name LIKE ? OR code LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if query.Status != nil {
		database = database.Where("status = ?", *query.Status)
	}
	var rows []departmentRow
	if err := database.Order("sort ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]domain.Department, len(rows))
	for index, row := range rows {
		items[index] = toDomain(row)
	}
	return items, nil
}

func (r *Repository) Get(ctx context.Context, id int64) (domain.Department, error) {
	var row departmentRow
	err := r.database.WithContext(ctx).Where("id = ? AND is_deleted = ?", id, 0).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Department{}, application.ErrNotFound
	}
	return toDomain(row), err
}

func (r *Repository) CodeExists(ctx context.Context, code string, excludeID int64) (bool, error) {
	return r.exists(ctx, "code = ? AND id <> ? AND is_deleted = 0", code, excludeID)
}

func (r *Repository) NameExists(ctx context.Context, name string, parentID, excludeID int64) (bool, error) {
	return r.exists(ctx, "name = ? AND parent_id = ? AND id <> ? AND is_deleted = 0", name, parentID, excludeID)
}

func (r *Repository) exists(ctx context.Context, condition string, args ...any) (bool, error) {
	var count int64
	err := r.database.WithContext(ctx).Model(&departmentRow{}).Where(condition, args...).Count(&count).Error
	return count > 0, err
}

func (r *Repository) IsDescendant(ctx context.Context, candidateParentID, departmentID int64) (bool, error) {
	var count int64
	pattern := "%," + number(departmentID) + ",%"
	err := r.database.WithContext(ctx).Model(&departmentRow{}).
		Where("id = ? AND CONCAT(',', tree_path, ',') LIKE ? AND is_deleted = 0", candidateParentID, pattern).Count(&count).Error
	return count > 0, err
}

func (r *Repository) HasChildren(ctx context.Context, id int64) (bool, error) {
	return r.exists(ctx, "parent_id = ? AND is_deleted = 0", id)
}

func (r *Repository) HasAccounts(ctx context.Context, id int64) (bool, error) {
	var count int64
	err := r.database.WithContext(ctx).Table("sys_user").Where("dept_id = ? AND is_deleted = 0", id).Count(&count).Error
	return count > 0, err
}

func (r *Repository) Create(ctx context.Context, item domain.Department, actorID int64) error {
	now := time.Now().UTC()
	row := departmentRow{Name: item.Name, Code: item.Code, ParentID: item.ParentID, TreePath: item.TreePath, Sort: item.Sort, Status: item.Status, CreateBy: &actorID, UpdateBy: &actorID, CreateTime: now, UpdateTime: now}
	return mysqladapter.NormalizeError(r.database.WithContext(ctx).Create(&row).Error)
}

func (r *Repository) UpdateTree(ctx context.Context, item domain.Department, oldParentPath string, actorID int64) error {
	return mysqladapter.NormalizeError(r.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		updates := map[string]any{"name": item.Name, "code": item.Code, "parent_id": item.ParentID, "tree_path": item.TreePath, "sort": item.Sort, "status": item.Status, "update_by": actorID, "update_time": time.Now().UTC()}
		result := transaction.Model(&departmentRow{}).Where("id = ? AND is_deleted = 0", item.ID).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return application.ErrNotFound
		}
		oldPrefix := oldParentPath + "," + number(item.ID)
		newPrefix := item.TreePath + "," + number(item.ID)
		return transaction.Model(&departmentRow{}).
			Where("CONCAT(',', tree_path, ',') LIKE ? AND is_deleted = 0", "%,"+number(item.ID)+",%").
			Update("tree_path", gorm.Expr("REPLACE(tree_path, ?, ?)", oldPrefix, newPrefix)).Error
	}))
}

func (r *Repository) Delete(ctx context.Context, id, actorID int64) error {
	result := r.database.WithContext(ctx).Model(&departmentRow{}).Where("id = ? AND is_deleted = 0", id).
		Updates(map[string]any{"is_deleted": 1, "update_by": actorID, "update_time": time.Now().UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return application.ErrNotFound
	}
	return nil
}

func toDomain(row departmentRow) domain.Department {
	return domain.Department{ID: row.ID, Name: row.Name, Code: row.Code, ParentID: row.ParentID, TreePath: row.TreePath, Sort: row.Sort, Status: row.Status, CreateTime: row.CreateTime, UpdateTime: row.UpdateTime}
}

func number(value int64) string {
	return fmt.Sprintf("%d", value)
}
