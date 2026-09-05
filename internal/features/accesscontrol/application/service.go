package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yuhang1130/go-service-main/internal/features/accesscontrol/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/apperror"
	"github.com/yuhang1130/go-service-main/internal/foundation/persistence"
)

var ErrNotFound = errors.New("not found")

type PageQuery struct {
	Page     int
	PageSize int
	Keywords string
}

type MenuQuery struct {
	Keywords string
	Status   *int
}

type RoleCommand struct {
	ID            int64
	Name          string
	Code          string
	Sort          int
	Status        int
	DataScope     int
	DepartmentIDs []int64
	MenuIDs       []int64
}

type MenuCommand struct {
	ID          int64
	ParentID    int64
	Name        string
	Type        string
	RouteName   string
	RoutePath   string
	Component   string
	ExternalURL string
	Permission  string
	AlwaysShow  int
	KeepAlive   int
	Visible     int
	Sort        int
	Icon        string
	Redirect    string
	Params      map[string]any
}

type Repository interface {
	Authorization(context.Context, int64) (domain.Authorization, error)
	RoleScopes(context.Context, int64) ([]domain.RoleScope, error)
	ListRoles(context.Context, PageQuery) ([]domain.Role, int64, error)
	AllRoles(context.Context) ([]domain.Role, error)
	GetRole(context.Context, int64) (domain.Role, error)
	RoleCodeExists(context.Context, string, int64) (bool, error)
	RoleNameExists(context.Context, string, int64) (bool, error)
	SaveRole(context.Context, domain.Role, int64) error
	DeleteRole(context.Context, int64, int64) error
	RoleInUse(context.Context, int64) (bool, error)
	RoleMenuIDs(context.Context, int64) ([]int64, error)
	SetRoleMenus(context.Context, int64, []int64, int64) error
	RoleDepartmentIDs(context.Context, int64) ([]int64, error)
	SetRoleDepartments(context.Context, int64, []int64, int64) error
	AccountIDsByRole(context.Context, int64) ([]int64, error)
	ListMenus(context.Context, MenuQuery) ([]domain.Menu, error)
	MenusForAccount(context.Context, int64, bool) ([]domain.Menu, error)
	GetMenu(context.Context, int64) (domain.Menu, error)
	MenuNameExists(context.Context, string, int64, int64) (bool, error)
	RouteNameExists(context.Context, string, int64) (bool, error)
	MenuIsDescendant(context.Context, int64, int64) (bool, error)
	CreateMenu(context.Context, domain.Menu, int64) error
	UpdateMenuTree(context.Context, domain.Menu, string, int64) error
	DeleteMenu(context.Context, int64, int64) error
	MenuHasChildren(context.Context, int64) (bool, error)
}

type SessionInvalidator interface {
	InvalidateUser(context.Context, int64) error
}

type Service struct {
	repository Repository
	sessions   SessionInvalidator
}

func NewService(repository Repository, sessions SessionInvalidator) *Service {
	return &Service{repository: repository, sessions: sessions}
}

func (s *Service) Authorization(ctx context.Context, accountID int64) (domain.Authorization, error) {
	authorization, err := s.repository.Authorization(ctx, accountID)
	if err != nil {
		return domain.Authorization{}, mapError(err, "账号不存在")
	}
	return authorization, nil
}

func (s *Service) Scope(ctx context.Context, accountID int64) (domain.AccountScope, error) {
	scopes, err := s.repository.RoleScopes(ctx, accountID)
	if err != nil {
		return domain.AccountScope{}, apperror.Internal(err)
	}
	return domain.MergeScopes(accountID, scopes), nil
}

func (s *Service) ListRoles(ctx context.Context, query PageQuery) ([]domain.Role, int64, error) {
	normalizePage(&query)
	items, total, err := s.repository.ListRoles(ctx, query)
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	return items, total, nil
}

func (s *Service) RoleOptions(ctx context.Context) ([]domain.Role, error) {
	items, err := s.repository.AllRoles(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return items, nil
}

func (s *Service) GetRole(ctx context.Context, id int64) (domain.Role, error) {
	role, err := s.repository.GetRole(ctx, id)
	if err != nil {
		return domain.Role{}, mapError(err, "角色不存在")
	}
	role.MenuIDs, err = s.repository.RoleMenuIDs(ctx, id)
	if err != nil {
		return domain.Role{}, apperror.Internal(err)
	}
	role.DepartmentIDs, err = s.repository.RoleDepartmentIDs(ctx, id)
	if err != nil {
		return domain.Role{}, apperror.Internal(err)
	}
	return role, nil
}

func (s *Service) SaveRole(ctx context.Context, command RoleCommand, actorID int64) error {
	role := domain.Role{ID: command.ID, Name: strings.TrimSpace(command.Name), Code: strings.ToUpper(strings.TrimSpace(command.Code)), Sort: command.Sort, Status: command.Status, DataScope: command.DataScope, DepartmentIDs: command.DepartmentIDs, MenuIDs: command.MenuIDs}
	if err := role.Validate(); err != nil {
		return apperror.InvalidArgument("A0400", "角色名称、编码、状态或数据权限无效", err)
	}
	if role.ID != 0 {
		current, err := s.repository.GetRole(ctx, role.ID)
		if err != nil {
			return mapError(err, "角色不存在")
		}
		if current.Code == domain.RootRoleCode && role.Code != domain.RootRoleCode {
			return apperror.Forbidden("A0300", "不能修改超级管理员角色编码")
		}
	}
	codeExists, err := s.repository.RoleCodeExists(ctx, role.Code, role.ID)
	if err != nil {
		return apperror.Internal(err)
	}
	nameExists, err := s.repository.RoleNameExists(ctx, role.Name, role.ID)
	if err != nil {
		return apperror.Internal(err)
	}
	if codeExists || nameExists {
		return apperror.Conflict("A0409", "角色名称或编码已存在")
	}
	if err := s.repository.SaveRole(ctx, role, actorID); err != nil {
		return mapConflict(err, "角色名称或编码已存在")
	}
	if role.ID != 0 {
		s.invalidateRoleSessions(ctx, role.ID)
	}
	return nil
}

func (s *Service) DeleteRole(ctx context.Context, id, actorID int64) error {
	role, err := s.repository.GetRole(ctx, id)
	if err != nil {
		return mapError(err, "角色不存在")
	}
	if role.Code == domain.RootRoleCode {
		return apperror.Forbidden("A0300", "超级管理员角色不能删除")
	}
	used, err := s.repository.RoleInUse(ctx, id)
	if err != nil {
		return apperror.Internal(err)
	}
	if used {
		return apperror.Conflict("A0409", "角色已分配给用户，不能删除")
	}
	if err := s.repository.DeleteRole(ctx, id, actorID); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

func (s *Service) RoleMenuIDs(ctx context.Context, id int64) ([]int64, error) {
	ids, err := s.repository.RoleMenuIDs(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return ids, nil
}

func (s *Service) SetRoleMenus(ctx context.Context, id int64, menuIDs []int64, actorID int64) error {
	if err := s.repository.SetRoleMenus(ctx, id, menuIDs, actorID); err != nil {
		return mapError(err, "角色不存在")
	}
	s.invalidateRoleSessions(ctx, id)
	return nil
}

func (s *Service) RoleDepartmentIDs(ctx context.Context, id int64) ([]int64, error) {
	ids, err := s.repository.RoleDepartmentIDs(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return ids, nil
}

func (s *Service) SetRoleDepartments(ctx context.Context, id int64, departmentIDs []int64, actorID int64) error {
	if err := s.repository.SetRoleDepartments(ctx, id, departmentIDs, actorID); err != nil {
		return mapError(err, "角色不存在")
	}
	s.invalidateRoleSessions(ctx, id)
	return nil
}

func (s *Service) ListMenus(ctx context.Context, query MenuQuery) ([]*domain.Menu, error) {
	items, err := s.repository.ListMenus(ctx, query)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return domain.BuildMenuTree(items), nil
}

func (s *Service) MenuOptions(ctx context.Context, parentOnly bool) ([]*domain.Menu, error) {
	items, err := s.repository.ListMenus(ctx, MenuQuery{})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if parentOnly {
		filtered := items[:0]
		for _, item := range items {
			if item.Type == "C" || item.Type == "M" {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	return domain.BuildMenuTree(items), nil
}

func (s *Service) Routes(ctx context.Context, accountID int64) ([]*domain.Route, error) {
	authorization, err := s.Authorization(ctx, accountID)
	if err != nil {
		return nil, err
	}
	items, err := s.repository.MenusForAccount(ctx, accountID, authorization.System)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return domain.BuildRoutes(items), nil
}

func (s *Service) GetMenu(ctx context.Context, id int64) (domain.Menu, error) {
	item, err := s.repository.GetMenu(ctx, id)
	if err != nil {
		return domain.Menu{}, mapError(err, "菜单不存在")
	}
	return item, nil
}

func (s *Service) SaveMenu(ctx context.Context, command MenuCommand, actorID int64) error {
	item := domain.Menu{ID: command.ID, ParentID: command.ParentID, Name: strings.TrimSpace(command.Name), Type: command.Type, RouteName: strings.TrimSpace(command.RouteName), RoutePath: strings.TrimSpace(command.RoutePath), Component: strings.TrimSpace(command.Component), ExternalURL: strings.TrimSpace(command.ExternalURL), Permission: strings.TrimSpace(command.Permission), AlwaysShow: command.AlwaysShow, KeepAlive: command.KeepAlive, Visible: command.Visible, Sort: command.Sort, Icon: command.Icon, Redirect: command.Redirect, Params: command.Params}
	if err := item.Validate(); err != nil {
		return apperror.InvalidArgument("A0400", "菜单名称、类型、权限或显示状态无效", err)
	}
	if item.ID != 0 && item.ParentID == item.ID {
		return apperror.InvalidArgument("A0400", "父级菜单不能是当前菜单", nil)
	}
	if item.ID != 0 && item.ParentID != 0 {
		descendant, err := s.repository.MenuIsDescendant(ctx, item.ParentID, item.ID)
		if err != nil {
			return apperror.Internal(err)
		}
		if descendant {
			return apperror.InvalidArgument("A0400", "不能将菜单移动到自己的下级", nil)
		}
	}
	nameExists, err := s.repository.MenuNameExists(ctx, item.Name, item.ParentID, item.ID)
	if err != nil {
		return apperror.Internal(err)
	}
	routeExists, err := s.repository.RouteNameExists(ctx, item.RouteName, item.ID)
	if err != nil {
		return apperror.Internal(err)
	}
	if nameExists || routeExists {
		return apperror.Conflict("A0409", "菜单名称或路由名称已存在")
	}
	item.TreePath = "0"
	if item.ParentID != 0 {
		parent, err := s.repository.GetMenu(ctx, item.ParentID)
		if err != nil {
			return mapError(err, "上级菜单不存在")
		}
		item.TreePath = fmt.Sprintf("%s,%d", parent.TreePath, parent.ID)
	}
	if item.ID == 0 {
		if err := s.repository.CreateMenu(ctx, item, actorID); err != nil {
			return mapConflict(err, "菜单名称或路由名称已存在")
		}
		return nil
	}
	current, err := s.repository.GetMenu(ctx, item.ID)
	if err != nil {
		return mapError(err, "菜单不存在")
	}
	if err := s.repository.UpdateMenuTree(ctx, item, current.TreePath, actorID); err != nil {
		return mapConflict(err, "菜单名称或路由名称已存在")
	}
	return nil
}

func (s *Service) DeleteMenu(ctx context.Context, id, actorID int64) error {
	hasChildren, err := s.repository.MenuHasChildren(ctx, id)
	if err != nil {
		return apperror.Internal(err)
	}
	if hasChildren {
		return apperror.Conflict("A0409", "菜单存在子菜单，不能删除")
	}
	if err := s.repository.DeleteMenu(ctx, id, actorID); err != nil {
		return mapError(err, "菜单不存在")
	}
	return nil
}

func (s *Service) invalidateRoleSessions(ctx context.Context, roleID int64) {
	if s.sessions == nil {
		return
	}
	accountIDs, err := s.repository.AccountIDsByRole(ctx, roleID)
	if err != nil {
		return
	}
	for _, accountID := range accountIDs {
		_ = s.sessions.InvalidateUser(ctx, accountID)
	}
}

func normalizePage(query *PageQuery) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 || query.PageSize > 200 {
		query.PageSize = 10
	}
}

func mapError(err error, message string) error {
	if errors.Is(err, ErrNotFound) {
		return apperror.NotFound("A0404", message)
	}
	return apperror.Internal(err)
}

func mapConflict(err error, message string) error {
	if errors.Is(err, persistence.ErrConflict) {
		return apperror.Conflict("A0409", message)
	}
	return apperror.Internal(err)
}
