package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	accessdomain "github.com/yuhang1130/go-service-main/internal/features/accesscontrol/domain"
	identityapp "github.com/yuhang1130/go-service-main/internal/features/identity/application"
	identitydomain "github.com/yuhang1130/go-service-main/internal/features/identity/domain"
	"gorm.io/gorm"
)

type Repository struct{ database *gorm.DB }

func NewRepository(database *gorm.DB) *Repository { return &Repository{database: database} }

type accountRow struct {
	ID           int64     `gorm:"column:id;primaryKey"`
	Username     string    `gorm:"column:username"`
	Nickname     string    `gorm:"column:nickname"`
	Gender       int       `gorm:"column:gender"`
	Password     string    `gorm:"column:password"`
	DepartmentID int64     `gorm:"column:dept_id"`
	Avatar       string    `gorm:"column:avatar"`
	Mobile       string    `gorm:"column:mobile"`
	Status       int       `gorm:"column:status"`
	Email        string    `gorm:"column:email"`
	CreateTime   time.Time `gorm:"column:create_time"`
	CreateBy     *int64    `gorm:"column:create_by"`
	UpdateTime   time.Time `gorm:"column:update_time"`
	UpdateBy     *int64    `gorm:"column:update_by"`
	IsDeleted    int       `gorm:"column:is_deleted"`
}

func (accountRow) TableName() string { return "sys_user" }

type accountViewRow struct {
	ID             int64     `gorm:"column:id"`
	Username       string    `gorm:"column:username"`
	Nickname       string    `gorm:"column:nickname"`
	Gender         int       `gorm:"column:gender"`
	Password       string    `gorm:"column:password"`
	DepartmentID   int64     `gorm:"column:dept_id"`
	Avatar         string    `gorm:"column:avatar"`
	Mobile         string    `gorm:"column:mobile"`
	Status         int       `gorm:"column:status"`
	Email          string    `gorm:"column:email"`
	CreateTime     time.Time `gorm:"column:create_time"`
	UpdateTime     time.Time `gorm:"column:update_time"`
	DepartmentName string    `gorm:"column:dept_name"`
	RoleNames      string    `gorm:"column:role_names"`
}

func (r *Repository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.database.WithContext(ctx).Model(&accountRow{}).Where("is_deleted = 0").Count(&count).Error
	return count, err
}

func (r *Repository) GetByUsername(ctx context.Context, username string) (identitydomain.Account, error) {
	var row accountRow
	err := r.database.WithContext(ctx).Where("username = ? AND is_deleted = 0", username).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return identitydomain.Account{}, identityapp.ErrNotFound
	}
	return accountToDomain(row), err
}

func (r *Repository) Get(ctx context.Context, id int64) (identitydomain.Account, error) {
	var row accountViewRow
	err := r.accountView(ctx).Where("account.id = ?", id).Group("account.id, department.name").Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return identitydomain.Account{}, identityapp.ErrNotFound
	}
	if err != nil {
		return identitydomain.Account{}, err
	}
	account := accountViewToDomain(row)
	if err := r.database.WithContext(ctx).Table("sys_user_role").Where("user_id = ?", id).Order("role_id ASC").Pluck("role_id", &account.RoleIDs).Error; err != nil {
		return identitydomain.Account{}, err
	}
	return account, nil
}

func (r *Repository) List(ctx context.Context, query identityapp.ListQuery, scope accessdomain.AccountScope) ([]identitydomain.Account, int64, error) {
	countDatabase := r.database.WithContext(ctx).Table("sys_user AS account").Where("account.is_deleted = 0")
	countDatabase = filterAccounts(countDatabase, query, scope)
	var total int64
	if err := countDatabase.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	database := filterAccounts(r.accountView(ctx), query, scope)
	var rows []accountViewRow
	err := database.Group("account.id, department.name").Order("account.create_time DESC, account.id DESC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	items := make([]identitydomain.Account, len(rows))
	for index, row := range rows {
		items[index] = accountViewToDomain(row)
	}
	return items, total, nil
}

func (r *Repository) Export(ctx context.Context, query identityapp.ListQuery, scope accessdomain.AccountScope, limit int) ([]identitydomain.Account, error) {
	var rows []accountViewRow
	database := filterAccounts(r.accountView(ctx), query, scope)
	if err := database.Group("account.id, department.name").Order("account.create_time DESC, account.id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]identitydomain.Account, len(rows))
	for index, row := range rows {
		items[index] = accountViewToDomain(row)
	}
	return items, nil
}

func filterAccounts(database *gorm.DB, query identityapp.ListQuery, scope accessdomain.AccountScope) *gorm.DB {
	database = applyScope(database, scope)
	if keyword := strings.TrimSpace(query.Keywords); keyword != "" {
		database = database.Where("account.username LIKE ? OR account.nickname LIKE ? OR account.mobile LIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if query.Status != nil {
		database = database.Where("account.status = ?", *query.Status)
	}
	if query.DepartmentID != nil {
		database = database.Where("account.dept_id = ?", *query.DepartmentID)
	}
	if query.CreatedFrom != nil {
		database = database.Where("account.create_time >= ?", *query.CreatedFrom)
	}
	if query.CreatedTo != nil {
		database = database.Where("account.create_time <= ?", *query.CreatedTo)
	}
	return database
}

func (r *Repository) Options(ctx context.Context, scope accessdomain.AccountScope) ([]identitydomain.Account, error) {
	var rows []accountRow
	database := r.database.WithContext(ctx).Table("sys_user AS account").Where("account.status = 1 AND account.is_deleted = 0")
	database = applyScope(database, scope)
	if err := database.Order("account.nickname ASC, account.id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]identitydomain.Account, len(rows))
	for index, row := range rows {
		items[index] = accountToDomain(row)
	}
	return items, nil
}

func (r *Repository) UsernameExists(ctx context.Context, username string, excludeID int64) (bool, error) {
	var count int64
	err := r.database.WithContext(ctx).Model(&accountRow{}).Where("username = ? AND id <> ? AND is_deleted = 0", username, excludeID).Count(&count).Error
	return count > 0, err
}

func (r *Repository) Save(ctx context.Context, account identitydomain.Account, roleIDs []int64, actorID int64) error {
	return mysqladapter.NormalizeError(r.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		now := time.Now().UTC()
		if account.ID == 0 {
			row := accountRow{Username: account.Username, Nickname: account.Nickname, Gender: account.Gender, Password: account.Password, DepartmentID: account.DepartmentID, Avatar: account.Avatar, Mobile: account.Mobile, Status: account.Status, Email: account.Email, CreateBy: &actorID, UpdateBy: &actorID, CreateTime: now, UpdateTime: now}
			if err := transaction.Create(&row).Error; err != nil {
				return err
			}
			account.ID = row.ID
		} else {
			result := transaction.Model(&accountRow{}).Where("id = ? AND is_deleted = 0", account.ID).Updates(map[string]any{"username": account.Username, "nickname": account.Nickname, "gender": account.Gender, "dept_id": nullableID(account.DepartmentID), "avatar": nullable(account.Avatar), "mobile": nullable(account.Mobile), "status": account.Status, "email": nullable(account.Email), "update_by": actorID, "update_time": now})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return identityapp.ErrNotFound
			}
		}
		if err := transaction.Exec("DELETE FROM sys_user_role WHERE user_id = ?", account.ID).Error; err != nil {
			return err
		}
		for _, roleID := range uniqueIDs(roleIDs) {
			if err := transaction.Exec("INSERT INTO sys_user_role (user_id, role_id) VALUES (?, ?)", account.ID, roleID).Error; err != nil {
				return err
			}
		}
		return nil
	}))
}

func (r *Repository) Delete(ctx context.Context, ids []int64, actorID int64) error {
	return r.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		result := transaction.Model(&accountRow{}).Where("id IN ? AND is_deleted = 0", ids).Updates(map[string]any{"is_deleted": 1, "update_by": actorID, "update_time": time.Now().UTC()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != int64(len(uniqueIDs(ids))) {
			return identityapp.ErrNotFound
		}
		return transaction.Exec("DELETE FROM sys_user_role WHERE user_id IN ?", ids).Error
	})
}

func (r *Repository) SetStatus(ctx context.Context, id int64, status int, actorID int64) error {
	return r.updateOne(ctx, id, map[string]any{"status": status, "update_by": actorID, "update_time": time.Now().UTC()})
}

func (r *Repository) SetPassword(ctx context.Context, id int64, password string, actorID int64) error {
	return r.updateOne(ctx, id, map[string]any{"password": password, "update_by": actorID, "update_time": time.Now().UTC()})
}

func (r *Repository) SetProfile(ctx context.Context, id int64, command identityapp.ProfileCommand) error {
	return r.updateOne(ctx, id, map[string]any{"nickname": command.Nickname, "avatar": nullable(command.Avatar), "gender": command.Gender, "update_by": id, "update_time": time.Now().UTC()})
}

func (r *Repository) updateOne(ctx context.Context, id int64, updates map[string]any) error {
	result := r.database.WithContext(ctx).Model(&accountRow{}).Where("id = ? AND is_deleted = 0", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return identityapp.ErrNotFound
	}
	return nil
}

func (r *Repository) Bootstrap(ctx context.Context, account identitydomain.Account, roleCode string) (bool, error) {
	created := false
	err := r.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var count int64
		if err := transaction.Model(&accountRow{}).Where("is_deleted = 0").Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return nil
		}
		var roleID int64
		if err := transaction.Table("sys_role").Where("code = ? AND status = 1 AND is_deleted = 0", roleCode).Pluck("id", &roleID).Error; err != nil {
			return err
		}
		if roleID == 0 {
			return errors.New("bootstrap role does not exist")
		}
		now := time.Now().UTC()
		row := accountRow{Username: account.Username, Nickname: account.Nickname, Gender: account.Gender, Password: account.Password, DepartmentID: account.DepartmentID, Status: account.Status, CreateTime: now, UpdateTime: now}
		if err := transaction.Create(&row).Error; err != nil {
			return err
		}
		result := transaction.Exec("INSERT INTO sys_user_role (user_id, role_id) VALUES (?, ?)", row.ID, roleID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("bootstrap role assignment affected %d rows", result.RowsAffected)
		}
		created = true
		return nil
	})
	return created, mysqladapter.NormalizeError(err)
}

func (r *Repository) IsRoot(ctx context.Context, id int64) (bool, error) {
	var count int64
	err := r.database.WithContext(ctx).Table("sys_user_role AS user_role").Joins("JOIN sys_role AS role ON role.id = user_role.role_id").Where("user_role.user_id = ? AND role.code = ? AND role.is_deleted = 0", id, accessdomain.RootRoleCode).Count(&count).Error
	return count > 0, err
}

func (r *Repository) ImportReferences(ctx context.Context) (identityapp.ImportReferences, error) {
	references := identityapp.ImportReferences{Roles: map[string]int64{}, Departments: map[string]int64{}}
	var roles []struct {
		ID   int64  `gorm:"column:id"`
		Name string `gorm:"column:name"`
		Code string `gorm:"column:code"`
	}
	if err := r.database.WithContext(ctx).Table("sys_role").Select("id, name, code").Where("status = 1 AND is_deleted = 0").Scan(&roles).Error; err != nil {
		return identityapp.ImportReferences{}, err
	}
	for _, role := range roles {
		references.Roles[role.Code] = role.ID
		references.Roles[role.Name] = role.ID
	}
	var departments []struct {
		ID   int64  `gorm:"column:id"`
		Name string `gorm:"column:name"`
		Code string `gorm:"column:code"`
	}
	if err := r.database.WithContext(ctx).Table("sys_dept").Select("id, name, code").Where("status = 1 AND is_deleted = 0").Scan(&departments).Error; err != nil {
		return identityapp.ImportReferences{}, err
	}
	for _, department := range departments {
		references.Departments[department.Code] = department.ID
		references.Departments[department.Name] = department.ID
	}
	return references, nil
}

func (r *Repository) Import(ctx context.Context, accounts []identitydomain.Account, actorID int64) error {
	return mysqladapter.NormalizeError(r.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		now := time.Now().UTC()
		for _, account := range accounts {
			row := accountRow{Username: account.Username, Nickname: account.Nickname, Gender: account.Gender, Password: account.Password, DepartmentID: account.DepartmentID, Mobile: account.Mobile, Status: account.Status, Email: account.Email, CreateBy: &actorID, UpdateBy: &actorID, CreateTime: now, UpdateTime: now}
			if err := transaction.Create(&row).Error; err != nil {
				return err
			}
			for _, roleID := range uniqueIDs(account.RoleIDs) {
				if err := transaction.Exec("INSERT INTO sys_user_role (user_id, role_id) VALUES (?, ?)", row.ID, roleID).Error; err != nil {
					return err
				}
			}
		}
		return nil
	}))
}

func (r *Repository) accountView(ctx context.Context) *gorm.DB {
	return r.database.WithContext(ctx).Table("sys_user AS account").
		Select("account.*, department.name AS dept_name, COALESCE(GROUP_CONCAT(DISTINCT role.name ORDER BY role.sort SEPARATOR ','), '') AS role_names").
		Joins("LEFT JOIN sys_dept AS department ON department.id = account.dept_id AND department.is_deleted = 0").
		Joins("LEFT JOIN sys_user_role AS user_role ON user_role.user_id = account.id").
		Joins("LEFT JOIN sys_role AS role ON role.id = user_role.role_id AND role.is_deleted = 0").
		Where("account.is_deleted = 0")
}

func applyScope(database *gorm.DB, scope accessdomain.AccountScope) *gorm.DB {
	if scope.All {
		return database
	}
	if len(scope.DepartmentIDs) == 0 {
		return database.Where("account.id = ?", scope.SelfID)
	}
	return database.Where("account.id = ? OR account.dept_id IN ?", scope.SelfID, scope.DepartmentIDs)
}

func accountToDomain(row accountRow) identitydomain.Account {
	return identitydomain.Account{ID: row.ID, Username: row.Username, Nickname: row.Nickname, Gender: row.Gender, Password: row.Password, DepartmentID: row.DepartmentID, Avatar: row.Avatar, Mobile: row.Mobile, Status: row.Status, Email: row.Email, CreateTime: row.CreateTime, UpdateTime: row.UpdateTime}
}

func accountViewToDomain(row accountViewRow) identitydomain.Account {
	return identitydomain.Account{
		ID: row.ID, Username: row.Username, Nickname: row.Nickname, Gender: row.Gender,
		Password: row.Password, DepartmentID: row.DepartmentID, Avatar: row.Avatar,
		Mobile: row.Mobile, Status: row.Status, Email: row.Email, CreateTime: row.CreateTime,
		UpdateTime: row.UpdateTime, DepartmentName: row.DepartmentName, RoleNames: row.RoleNames,
	}
}

func uniqueIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				result = append(result, id)
			}
		}
	}
	return result
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableID(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}
