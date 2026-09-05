package accesscontrol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	mysqladapter "github.com/yuhang1130/go-service-main/internal/adapters/mysql"
	"github.com/yuhang1130/go-service-main/internal/features/accesscontrol/application"
	"github.com/yuhang1130/go-service-main/internal/features/accesscontrol/domain"
	"gorm.io/gorm"
)

type Repository struct{ database *gorm.DB }

func NewRepository(database *gorm.DB) *Repository { return &Repository{database: database} }

type roleRow struct {
	ID         int64     `gorm:"column:id;primaryKey"`
	Name       string    `gorm:"column:name"`
	Code       string    `gorm:"column:code"`
	Sort       int       `gorm:"column:sort"`
	Status     int       `gorm:"column:status"`
	DataScope  int       `gorm:"column:data_scope"`
	CreateBy   *int64    `gorm:"column:create_by"`
	CreateTime time.Time `gorm:"column:create_time"`
	UpdateBy   *int64    `gorm:"column:update_by"`
	UpdateTime time.Time `gorm:"column:update_time"`
	IsDeleted  int       `gorm:"column:is_deleted"`
}

func (roleRow) TableName() string { return "sys_role" }

type menuRow struct {
	ID          int64     `gorm:"column:id;primaryKey"`
	ParentID    int64     `gorm:"column:parent_id"`
	TreePath    string    `gorm:"column:tree_path"`
	Name        string    `gorm:"column:name"`
	Type        string    `gorm:"column:type"`
	RouteName   string    `gorm:"column:route_name"`
	RoutePath   string    `gorm:"column:route_path"`
	Component   string    `gorm:"column:component"`
	ExternalURL string    `gorm:"column:external_url"`
	Permission  string    `gorm:"column:perm"`
	AlwaysShow  int       `gorm:"column:always_show"`
	KeepAlive   int       `gorm:"column:keep_alive"`
	Visible     int       `gorm:"column:visible"`
	Sort        int       `gorm:"column:sort"`
	Icon        string    `gorm:"column:icon"`
	Redirect    string    `gorm:"column:redirect"`
	Params      []byte    `gorm:"column:params"`
	CreateTime  time.Time `gorm:"column:create_time"`
	UpdateTime  time.Time `gorm:"column:update_time"`
}

func (menuRow) TableName() string { return "sys_menu" }

func (r *Repository) Authorization(ctx context.Context, accountID int64) (domain.Authorization, error) {
	var roles []string
	err := r.database.WithContext(ctx).Table("sys_role AS role").
		Select("DISTINCT role.code").
		Joins("JOIN sys_user_role AS user_role ON user_role.role_id = role.id").
		Where("user_role.user_id = ? AND role.status = 1 AND role.is_deleted = 0", accountID).
		Order("role.code ASC").Pluck("role.code", &roles).Error
	if err != nil {
		return domain.Authorization{}, err
	}
	if len(roles) == 0 {
		return domain.Authorization{}, application.ErrNotFound
	}
	authorization := domain.Authorization{Roles: roles}
	for _, role := range roles {
		if role == domain.RootRoleCode {
			authorization.System = true
			return authorization, nil
		}
	}
	err = r.database.WithContext(ctx).Table("sys_menu AS menu").
		Select("DISTINCT menu.perm").
		Joins("JOIN sys_role_menu AS role_menu ON role_menu.menu_id = menu.id").
		Joins("JOIN sys_role AS role ON role.id = role_menu.role_id").
		Joins("JOIN sys_user_role AS user_role ON user_role.role_id = role.id").
		Where("user_role.user_id = ? AND role.status = 1 AND role.is_deleted = 0 AND menu.perm IS NOT NULL AND menu.perm <> ''", accountID).
		Order("menu.perm ASC").Pluck("menu.perm", &authorization.Permissions).Error
	return authorization, err
}

func (r *Repository) RoleScopes(ctx context.Context, accountID int64) ([]domain.RoleScope, error) {
	var account struct {
		DepartmentID int64 `gorm:"column:dept_id"`
	}
	err := r.database.WithContext(ctx).Table("sys_user").Select("dept_id").Where("id = ? AND is_deleted = 0", accountID).Take(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, application.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var roles []roleRow
	if err := r.database.WithContext(ctx).Table("sys_role AS role").
		Joins("JOIN sys_user_role AS user_role ON user_role.role_id = role.id").
		Where("user_role.user_id = ? AND role.status = 1 AND role.is_deleted = 0", accountID).Find(&roles).Error; err != nil {
		return nil, err
	}
	result := make([]domain.RoleScope, 0, len(roles))
	for _, role := range roles {
		scope := domain.RoleScope{Kind: role.DataScope}
		switch role.DataScope {
		case domain.ScopeAll:
			return []domain.RoleScope{{Kind: domain.ScopeAll}}, nil
		case domain.ScopeDepartment:
			if account.DepartmentID != 0 {
				scope.DepartmentIDs = []int64{account.DepartmentID}
			}
		case domain.ScopeDepartmentTree:
			if account.DepartmentID != 0 {
				if err := r.database.WithContext(ctx).Table("sys_dept").Where("(id = ? OR CONCAT(',', tree_path, ',') LIKE ?) AND is_deleted = 0", account.DepartmentID, "%,"+fmt.Sprint(account.DepartmentID)+",%").Pluck("id", &scope.DepartmentIDs).Error; err != nil {
					return nil, err
				}
			}
		case domain.ScopeCustom:
			if err := r.database.WithContext(ctx).Table("sys_role_dept").Where("role_id = ?", role.ID).Pluck("dept_id", &scope.DepartmentIDs).Error; err != nil {
				return nil, err
			}
		}
		result = append(result, scope)
	}
	return result, nil
}

func (r *Repository) ListRoles(ctx context.Context, query application.PageQuery) ([]domain.Role, int64, error) {
	database := r.database.WithContext(ctx).Model(&roleRow{}).Where("is_deleted = 0")
	if keyword := strings.TrimSpace(query.Keywords); keyword != "" {
		database = database.Where("name LIKE ? OR code LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	var total int64
	if err := database.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []roleRow
	if err := database.Order("sort ASC, id ASC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rolesToDomain(rows), total, nil
}

func (r *Repository) AllRoles(ctx context.Context) ([]domain.Role, error) {
	var rows []roleRow
	err := r.database.WithContext(ctx).Where("status = 1 AND is_deleted = 0").Order("sort ASC, id ASC").Find(&rows).Error
	return rolesToDomain(rows), err
}

func (r *Repository) GetRole(ctx context.Context, id int64) (domain.Role, error) {
	var row roleRow
	err := r.database.WithContext(ctx).Where("id = ? AND is_deleted = 0", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Role{}, application.ErrNotFound
	}
	return roleToDomain(row), err
}

func (r *Repository) RoleCodeExists(ctx context.Context, code string, excludeID int64) (bool, error) {
	return r.roleExists(ctx, "code = ? AND id <> ? AND is_deleted = 0", code, excludeID)
}

func (r *Repository) RoleNameExists(ctx context.Context, name string, excludeID int64) (bool, error) {
	return r.roleExists(ctx, "name = ? AND id <> ? AND is_deleted = 0", name, excludeID)
}

func (r *Repository) roleExists(ctx context.Context, condition string, args ...any) (bool, error) {
	var count int64
	err := r.database.WithContext(ctx).Model(&roleRow{}).Where(condition, args...).Count(&count).Error
	return count > 0, err
}

func (r *Repository) SaveRole(ctx context.Context, role domain.Role, actorID int64) error {
	return mysqladapter.NormalizeError(r.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		now := time.Now().UTC()
		if role.ID == 0 {
			row := roleRow{Name: role.Name, Code: role.Code, Sort: role.Sort, Status: role.Status, DataScope: role.DataScope, CreateBy: &actorID, UpdateBy: &actorID, CreateTime: now, UpdateTime: now}
			if err := transaction.Create(&row).Error; err != nil {
				return err
			}
			role.ID = row.ID
		} else {
			result := transaction.Model(&roleRow{}).Where("id = ? AND is_deleted = 0", role.ID).Updates(map[string]any{"name": role.Name, "code": role.Code, "sort": role.Sort, "status": role.Status, "data_scope": role.DataScope, "update_by": actorID, "update_time": now})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return application.ErrNotFound
			}
		}
		if err := replaceIDs(transaction, "sys_role_menu", "role_id", "menu_id", role.ID, role.MenuIDs); err != nil {
			return err
		}
		return replaceIDs(transaction, "sys_role_dept", "role_id", "dept_id", role.ID, role.DepartmentIDs)
	}))
}

func (r *Repository) DeleteRole(ctx context.Context, id, actorID int64) error {
	return r.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		result := transaction.Model(&roleRow{}).Where("id = ? AND is_deleted = 0", id).Updates(map[string]any{"is_deleted": 1, "update_by": actorID, "update_time": time.Now().UTC()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return application.ErrNotFound
		}
		if err := transaction.Exec("DELETE FROM sys_role_menu WHERE role_id = ?", id).Error; err != nil {
			return err
		}
		return transaction.Exec("DELETE FROM sys_role_dept WHERE role_id = ?", id).Error
	})
}

func (r *Repository) RoleInUse(ctx context.Context, id int64) (bool, error) {
	var count int64
	err := r.database.WithContext(ctx).Table("sys_user_role").Where("role_id = ?", id).Count(&count).Error
	return count > 0, err
}

func (r *Repository) RoleMenuIDs(ctx context.Context, id int64) ([]int64, error) {
	var ids []int64
	err := r.database.WithContext(ctx).Table("sys_role_menu").Where("role_id = ?", id).Order("menu_id ASC").Pluck("menu_id", &ids).Error
	return ids, err
}

func (r *Repository) SetRoleMenus(ctx context.Context, id int64, ids []int64, _ int64) error {
	return r.replaceRoleIDs(ctx, id, "sys_role_menu", "menu_id", ids)
}

func (r *Repository) RoleDepartmentIDs(ctx context.Context, id int64) ([]int64, error) {
	var ids []int64
	err := r.database.WithContext(ctx).Table("sys_role_dept").Where("role_id = ?", id).Order("dept_id ASC").Pluck("dept_id", &ids).Error
	return ids, err
}

func (r *Repository) SetRoleDepartments(ctx context.Context, id int64, ids []int64, _ int64) error {
	return r.replaceRoleIDs(ctx, id, "sys_role_dept", "dept_id", ids)
}

func (r *Repository) replaceRoleIDs(ctx context.Context, roleID int64, table, targetColumn string, ids []int64) error {
	return r.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var count int64
		if err := transaction.Model(&roleRow{}).Where("id = ? AND is_deleted = 0", roleID).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return application.ErrNotFound
		}
		return replaceIDs(transaction, table, "role_id", targetColumn, roleID, ids)
	})
}

func replaceIDs(transaction *gorm.DB, table, ownerColumn, targetColumn string, ownerID int64, ids []int64) error {
	deleteSQL, ok := map[string]string{
		"sys_role_menu": "DELETE FROM sys_role_menu WHERE role_id = ?",
		"sys_role_dept": "DELETE FROM sys_role_dept WHERE role_id = ?",
	}[table]
	if !ok || ownerColumn != "role_id" {
		return errors.New("unsupported role association")
	}
	if err := transaction.Exec(deleteSQL, ownerID).Error; err != nil {
		return err
	}
	for _, id := range uniqueIDs(ids) {
		insertSQL, ok := map[string]string{
			"menu_id": "INSERT INTO sys_role_menu (role_id, menu_id) VALUES (?, ?)",
			"dept_id": "INSERT INTO sys_role_dept (role_id, dept_id) VALUES (?, ?)",
		}[targetColumn]
		if !ok {
			return errors.New("unsupported role association target")
		}
		if err := transaction.Exec(insertSQL, ownerID, id).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) AccountIDsByRole(ctx context.Context, roleID int64) ([]int64, error) {
	var ids []int64
	err := r.database.WithContext(ctx).Table("sys_user_role").Where("role_id = ?", roleID).Pluck("user_id", &ids).Error
	return ids, err
}

func (r *Repository) ListMenus(ctx context.Context, query application.MenuQuery) ([]domain.Menu, error) {
	database := r.database.WithContext(ctx).Model(&menuRow{})
	if keyword := strings.TrimSpace(query.Keywords); keyword != "" {
		database = database.Where("name LIKE ?", "%"+keyword+"%")
	}
	if query.Status != nil {
		database = database.Where("visible = ?", *query.Status)
	}
	var rows []menuRow
	if err := database.Order("sort ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return menusToDomain(rows), nil
}

func (r *Repository) MenusForAccount(ctx context.Context, accountID int64, system bool) ([]domain.Menu, error) {
	database := r.database.WithContext(ctx).Model(&menuRow{}).Where("visible = 1")
	if !system {
		database = database.Where("id IN (?)", r.database.WithContext(ctx).Table("sys_role_menu AS role_menu").Select("role_menu.menu_id").Joins("JOIN sys_user_role AS user_role ON user_role.role_id = role_menu.role_id").Joins("JOIN sys_role AS role ON role.id = role_menu.role_id").Where("user_role.user_id = ? AND role.status = 1 AND role.is_deleted = 0", accountID))
	}
	var rows []menuRow
	err := database.Order("sort ASC, id ASC").Find(&rows).Error
	return menusToDomain(rows), err
}

func (r *Repository) GetMenu(ctx context.Context, id int64) (domain.Menu, error) {
	var row menuRow
	err := r.database.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Menu{}, application.ErrNotFound
	}
	return menuToDomain(row), err
}

func (r *Repository) MenuNameExists(ctx context.Context, name string, parentID, excludeID int64) (bool, error) {
	var count int64
	err := r.database.WithContext(ctx).Model(&menuRow{}).Where("name = ? AND parent_id = ? AND id <> ?", name, parentID, excludeID).Count(&count).Error
	return count > 0, err
}

func (r *Repository) RouteNameExists(ctx context.Context, name string, excludeID int64) (bool, error) {
	if strings.TrimSpace(name) == "" {
		return false, nil
	}
	var count int64
	err := r.database.WithContext(ctx).Model(&menuRow{}).Where("route_name = ? AND id <> ?", name, excludeID).Count(&count).Error
	return count > 0, err
}

func (r *Repository) MenuIsDescendant(ctx context.Context, candidateParentID, menuID int64) (bool, error) {
	var count int64
	pattern := "%," + fmt.Sprint(menuID) + ",%"
	err := r.database.WithContext(ctx).Model(&menuRow{}).
		Where("id = ? AND CONCAT(',', tree_path, ',') LIKE ?", candidateParentID, pattern).
		Count(&count).Error
	return count > 0, err
}

func (r *Repository) CreateMenu(ctx context.Context, item domain.Menu, _ int64) error {
	row, err := menuFromDomain(item)
	if err != nil {
		return err
	}
	return mysqladapter.NormalizeError(r.database.WithContext(ctx).Create(&row).Error)
}

func (r *Repository) UpdateMenuTree(ctx context.Context, item domain.Menu, oldParentPath string, _ int64) error {
	params, err := json.Marshal(item.Params)
	if err != nil {
		return err
	}
	return mysqladapter.NormalizeError(r.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		updates := map[string]any{"parent_id": item.ParentID, "tree_path": item.TreePath, "name": item.Name, "type": item.Type, "route_name": nullable(item.RouteName), "route_path": nullable(item.RoutePath), "component": nullable(item.Component), "external_url": nullable(item.ExternalURL), "perm": nullable(item.Permission), "always_show": item.AlwaysShow, "keep_alive": item.KeepAlive, "visible": item.Visible, "sort": item.Sort, "icon": nullable(item.Icon), "redirect": nullable(item.Redirect), "params": nullableBytes(params), "update_time": time.Now().UTC()}
		result := transaction.Model(&menuRow{}).Where("id = ?", item.ID).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return application.ErrNotFound
		}
		oldPrefix := oldParentPath + "," + fmt.Sprint(item.ID)
		newPrefix := item.TreePath + "," + fmt.Sprint(item.ID)
		return transaction.Model(&menuRow{}).Where("CONCAT(',', tree_path, ',') LIKE ?", "%,"+fmt.Sprint(item.ID)+",%").Update("tree_path", gorm.Expr("REPLACE(tree_path, ?, ?)", oldPrefix, newPrefix)).Error
	}))
}

func (r *Repository) DeleteMenu(ctx context.Context, id, _ int64) error {
	return r.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if err := transaction.Exec("DELETE FROM sys_role_menu WHERE menu_id = ?", id).Error; err != nil {
			return err
		}
		result := transaction.Where("id = ?", id).Delete(&menuRow{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return application.ErrNotFound
		}
		return nil
	})
}

func (r *Repository) MenuHasChildren(ctx context.Context, id int64) (bool, error) {
	var count int64
	err := r.database.WithContext(ctx).Model(&menuRow{}).Where("parent_id = ?", id).Count(&count).Error
	return count > 0, err
}

func roleToDomain(row roleRow) domain.Role {
	return domain.Role{ID: row.ID, Name: row.Name, Code: row.Code, Sort: row.Sort, Status: row.Status, DataScope: row.DataScope, CreateTime: row.CreateTime, UpdateTime: row.UpdateTime}
}

func rolesToDomain(rows []roleRow) []domain.Role {
	items := make([]domain.Role, len(rows))
	for index, row := range rows {
		items[index] = roleToDomain(row)
	}
	return items
}

func menuToDomain(row menuRow) domain.Menu {
	var params map[string]any
	if len(row.Params) > 0 {
		_ = json.Unmarshal(row.Params, &params)
	}
	return domain.Menu{ID: row.ID, ParentID: row.ParentID, TreePath: row.TreePath, Name: row.Name, Type: row.Type, RouteName: row.RouteName, RoutePath: row.RoutePath, Component: row.Component, ExternalURL: row.ExternalURL, Permission: row.Permission, AlwaysShow: row.AlwaysShow, KeepAlive: row.KeepAlive, Visible: row.Visible, Sort: row.Sort, Icon: row.Icon, Redirect: row.Redirect, Params: params, CreateTime: row.CreateTime, UpdateTime: row.UpdateTime}
}

func menusToDomain(rows []menuRow) []domain.Menu {
	items := make([]domain.Menu, len(rows))
	for index, row := range rows {
		items[index] = menuToDomain(row)
	}
	return items
}

func menuFromDomain(item domain.Menu) (menuRow, error) {
	params, err := json.Marshal(item.Params)
	if err != nil {
		return menuRow{}, err
	}
	now := time.Now().UTC()
	return menuRow{ParentID: item.ParentID, TreePath: item.TreePath, Name: item.Name, Type: item.Type, RouteName: item.RouteName, RoutePath: item.RoutePath, Component: item.Component, ExternalURL: item.ExternalURL, Permission: item.Permission, AlwaysShow: item.AlwaysShow, KeepAlive: item.KeepAlive, Visible: item.Visible, Sort: item.Sort, Icon: item.Icon, Redirect: item.Redirect, Params: params, CreateTime: now, UpdateTime: now}, nil
}

func uniqueIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			result = append(result, id)
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

func nullableBytes(value []byte) any {
	if string(value) == "null" || len(value) == 0 {
		return nil
	}
	return value
}
