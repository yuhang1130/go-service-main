package dictionary

import (
	"context"
	"errors"
	"strings"
	"time"

	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	dictionaryapp "github.com/yuhang1130/go-service-main/internal/features/dictionary/application"
	dictionarydomain "github.com/yuhang1130/go-service-main/internal/features/dictionary/domain"
	"gorm.io/gorm"
)

type Repository struct{ database *gorm.DB }

func NewRepository(database *gorm.DB) *Repository { return &Repository{database: database} }

type dictionaryRow struct {
	ID         int64     `gorm:"column:id;primaryKey"`
	DictCode   string    `gorm:"column:dict_code"`
	Name       string    `gorm:"column:name"`
	Status     int       `gorm:"column:status"`
	Remark     string    `gorm:"column:remark"`
	CreateBy   *int64    `gorm:"column:create_by"`
	CreateTime time.Time `gorm:"column:create_time"`
	UpdateBy   *int64    `gorm:"column:update_by"`
	UpdateTime time.Time `gorm:"column:update_time"`
	IsDeleted  int       `gorm:"column:is_deleted"`
}

func (dictionaryRow) TableName() string { return "sys_dict" }

type itemRow struct {
	ID         int64     `gorm:"column:id;primaryKey"`
	DictCode   string    `gorm:"column:dict_code"`
	Value      string    `gorm:"column:value"`
	Label      string    `gorm:"column:label"`
	TagType    string    `gorm:"column:tag_type"`
	Sort       int       `gorm:"column:sort"`
	Status     int       `gorm:"column:status"`
	Remark     string    `gorm:"column:remark"`
	CreateBy   *int64    `gorm:"column:create_by"`
	CreateTime time.Time `gorm:"column:create_time"`
	UpdateBy   *int64    `gorm:"column:update_by"`
	UpdateTime time.Time `gorm:"column:update_time"`
}

func (itemRow) TableName() string { return "sys_dict_item" }

func (r *Repository) List(ctx context.Context, query dictionaryapp.Query) ([]dictionarydomain.Dictionary, int64, error) {
	database := r.database.WithContext(ctx).Model(&dictionaryRow{}).Where("is_deleted = 0")
	if query.Keywords != "" {
		keyword := "%" + query.Keywords + "%"
		database = database.Where("dict_code LIKE ? OR name LIKE ?", keyword, keyword)
	}
	if query.Status != nil {
		database = database.Where("status = ?", *query.Status)
	}
	var total int64
	if err := database.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []dictionaryRow
	if err := database.Order("create_time DESC, id DESC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return dictionariesToDomain(rows), total, nil
}

func (r *Repository) Options(ctx context.Context) ([]dictionarydomain.Dictionary, error) {
	var rows []dictionaryRow
	if err := r.database.WithContext(ctx).Where("status = 1 AND is_deleted = 0").Order("name ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return dictionariesToDomain(rows), nil
}

func (r *Repository) Get(ctx context.Context, id int64) (dictionarydomain.Dictionary, error) {
	var row dictionaryRow
	err := r.database.WithContext(ctx).Where("id = ? AND is_deleted = 0", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return dictionarydomain.Dictionary{}, dictionaryapp.ErrNotFound
	}
	return dictionaryToDomain(row), err
}

func (r *Repository) DictionaryExists(ctx context.Context, code string) (bool, error) {
	return r.exists(ctx, &dictionaryRow{}, "dict_code = ? AND is_deleted = 0", code)
}

func (r *Repository) CodeExists(ctx context.Context, code string, excludeID int64) (bool, error) {
	return r.exists(ctx, &dictionaryRow{}, "dict_code = ? AND id <> ? AND is_deleted = 0", code, excludeID)
}

func (r *Repository) Create(ctx context.Context, item dictionarydomain.Dictionary, actorID int64) error {
	now := time.Now().UTC()
	row := dictionaryRow{DictCode: item.Code, Name: item.Name, Status: item.Status, Remark: item.Remark, CreateBy: &actorID, UpdateBy: &actorID, CreateTime: now, UpdateTime: now}
	return mysqladapter.NormalizeError(r.database.WithContext(ctx).Create(&row).Error)
}

func (r *Repository) Update(ctx context.Context, item dictionarydomain.Dictionary, actorID int64) error {
	return mysqladapter.NormalizeError(r.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var current dictionaryRow
		if err := transaction.Where("id = ? AND is_deleted = 0", item.ID).First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return dictionaryapp.ErrNotFound
			}
			return err
		}
		result := transaction.Model(&dictionaryRow{}).Where("id = ? AND is_deleted = 0", item.ID).Updates(map[string]any{
			"dict_code": item.Code, "name": item.Name, "status": item.Status, "remark": item.Remark,
			"update_by": actorID, "update_time": time.Now().UTC(),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return dictionaryapp.ErrNotFound
		}
		if current.DictCode == item.Code {
			return nil
		}
		return transaction.Model(&itemRow{}).Where("dict_code = ?", current.DictCode).Update("dict_code", item.Code).Error
	}))
}

func (r *Repository) Delete(ctx context.Context, id, actorID int64) error {
	result := r.database.WithContext(ctx).Model(&dictionaryRow{}).Where("id = ? AND is_deleted = 0", id).Updates(map[string]any{
		"is_deleted": 1, "update_by": actorID, "update_time": time.Now().UTC(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return dictionaryapp.ErrNotFound
	}
	return nil
}

func (r *Repository) ItemCount(ctx context.Context, code string) (int64, error) {
	var count int64
	err := r.database.WithContext(ctx).Model(&itemRow{}).Where("dict_code = ?", code).Count(&count).Error
	return count, err
}

func (r *Repository) ListItems(ctx context.Context, query dictionaryapp.ItemQuery) ([]dictionarydomain.Item, int64, error) {
	database := r.database.WithContext(ctx).Model(&itemRow{}).Where("dict_code = ?", query.DictCode)
	if query.Keywords != "" {
		keyword := "%" + query.Keywords + "%"
		database = database.Where("label LIKE ? OR value LIKE ?", keyword, keyword)
	}
	if query.Status != nil {
		database = database.Where("status = ?", *query.Status)
	}
	var total int64
	if err := database.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []itemRow
	if err := database.Order("sort ASC, id ASC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return itemsToDomain(rows), total, nil
}

func (r *Repository) ItemOptions(ctx context.Context, code string) ([]dictionarydomain.Item, error) {
	var rows []itemRow
	if err := r.database.WithContext(ctx).Where("dict_code = ? AND status = 1", code).Order("sort ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return itemsToDomain(rows), nil
}

func (r *Repository) GetItem(ctx context.Context, id int64, code string) (dictionarydomain.Item, error) {
	var row itemRow
	err := r.database.WithContext(ctx).Where("id = ? AND dict_code = ?", id, code).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return dictionarydomain.Item{}, dictionaryapp.ErrNotFound
	}
	return itemToDomain(row), err
}

func (r *Repository) ItemValueExists(ctx context.Context, code, value string, excludeID int64) (bool, error) {
	return r.exists(ctx, &itemRow{}, "dict_code = ? AND value = ? AND id <> ?", code, value, excludeID)
}

func (r *Repository) CreateItem(ctx context.Context, item dictionarydomain.Item, actorID int64) error {
	now := time.Now().UTC()
	row := itemRow{DictCode: item.DictCode, Value: item.Value, Label: item.Label, TagType: item.TagType, Sort: item.Sort, Status: item.Status, Remark: item.Remark, CreateBy: &actorID, UpdateBy: &actorID, CreateTime: now, UpdateTime: now}
	return mysqladapter.NormalizeError(r.database.WithContext(ctx).Create(&row).Error)
}

func (r *Repository) UpdateItem(ctx context.Context, item dictionarydomain.Item, actorID int64) error {
	result := r.database.WithContext(ctx).Model(&itemRow{}).Where("id = ? AND dict_code = ?", item.ID, item.DictCode).Updates(map[string]any{
		"value": item.Value, "label": item.Label, "tag_type": item.TagType, "sort": item.Sort, "status": item.Status,
		"remark": item.Remark, "update_by": actorID, "update_time": time.Now().UTC(),
	})
	if result.Error != nil {
		return mysqladapter.NormalizeError(result.Error)
	}
	if result.RowsAffected != 1 {
		return dictionaryapp.ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteItems(ctx context.Context, code string, ids []int64) error {
	return r.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var count int64
		if err := transaction.Model(&itemRow{}).Where("dict_code = ? AND id IN ?", code, ids).Count(&count).Error; err != nil {
			return err
		}
		if count != int64(len(ids)) {
			return dictionaryapp.ErrNotFound
		}
		result := transaction.Where("dict_code = ? AND id IN ?", code, ids).Delete(&itemRow{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != int64(len(ids)) {
			return dictionaryapp.ErrNotFound
		}
		return nil
	})
}

func (r *Repository) exists(ctx context.Context, model any, condition string, args ...any) (bool, error) {
	var count int64
	err := r.database.WithContext(ctx).Model(model).Where(condition, args...).Count(&count).Error
	return count > 0, err
}

func dictionariesToDomain(rows []dictionaryRow) []dictionarydomain.Dictionary {
	items := make([]dictionarydomain.Dictionary, len(rows))
	for index, row := range rows {
		items[index] = dictionaryToDomain(row)
	}
	return items
}

func dictionaryToDomain(row dictionaryRow) dictionarydomain.Dictionary {
	return dictionarydomain.Dictionary{ID: row.ID, Code: row.DictCode, Name: row.Name, Status: row.Status, Remark: row.Remark, CreateTime: row.CreateTime, UpdateTime: row.UpdateTime}
}

func itemsToDomain(rows []itemRow) []dictionarydomain.Item {
	items := make([]dictionarydomain.Item, len(rows))
	for index, row := range rows {
		items[index] = itemToDomain(row)
	}
	return items
}

func itemToDomain(row itemRow) dictionarydomain.Item {
	return dictionarydomain.Item{ID: row.ID, DictCode: row.DictCode, Value: row.Value, Label: row.Label, TagType: normalizeTagType(row.TagType), Sort: row.Sort, Status: row.Status, Remark: row.Remark, CreateTime: row.CreateTime, UpdateTime: row.UpdateTime}
}

func normalizeTagType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "primary", "p":
		return "P"
	case "success", "s":
		return "S"
	case "warning", "w":
		return "W"
	case "info", "i":
		return "I"
	case "danger", "error", "d":
		return "D"
	default:
		return "N"
	}
}
