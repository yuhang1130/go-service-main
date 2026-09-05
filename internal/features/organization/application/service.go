package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yuhang1130/go-service-main/internal/features/organization/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/apperror"
	"github.com/yuhang1130/go-service-main/internal/foundation/persistence"
)

type Query struct {
	Keywords string
	Status   *int
}

type SaveCommand struct {
	ID       int64
	Name     string
	Code     string
	ParentID int64
	Sort     int
	Status   int
}

type Repository interface {
	List(context.Context, Query) ([]domain.Department, error)
	Get(context.Context, int64) (domain.Department, error)
	CodeExists(context.Context, string, int64) (bool, error)
	NameExists(context.Context, string, int64, int64) (bool, error)
	IsDescendant(context.Context, int64, int64) (bool, error)
	HasChildren(context.Context, int64) (bool, error)
	HasAccounts(context.Context, int64) (bool, error)
	Create(context.Context, domain.Department, int64) error
	UpdateTree(context.Context, domain.Department, string, int64) error
	Delete(context.Context, int64, int64) error
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) List(ctx context.Context, query Query) ([]*domain.Department, error) {
	items, err := s.repository.List(ctx, query)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return domain.BuildTree(items), nil
}

func (s *Service) Options(ctx context.Context) ([]*domain.Department, error) {
	return s.List(ctx, Query{Status: intPointer(1)})
}

func (s *Service) Get(ctx context.Context, id int64) (domain.Department, error) {
	item, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.Department{}, mapNotFound(err, "部门不存在")
	}
	return item, nil
}

func (s *Service) Create(ctx context.Context, command SaveCommand, actorID int64) error {
	item, err := s.prepare(ctx, command)
	if err != nil {
		return err
	}
	if err := s.repository.Create(ctx, item, actorID); err != nil {
		return mapConflict(err, "部门编码已存在")
	}
	return nil
}

func (s *Service) Update(ctx context.Context, command SaveCommand, actorID int64) error {
	if command.ID <= 0 || command.ParentID == command.ID {
		return apperror.InvalidArgument("A0400", "部门层级无效", nil)
	}
	current, err := s.repository.Get(ctx, command.ID)
	if err != nil {
		return mapNotFound(err, "部门不存在")
	}
	if command.ParentID != 0 {
		descendant, err := s.repository.IsDescendant(ctx, command.ParentID, command.ID)
		if err != nil {
			return apperror.Internal(err)
		}
		if descendant {
			return apperror.InvalidArgument("A0400", "不能将部门移动到自己的下级", nil)
		}
	}
	item, err := s.prepare(ctx, command)
	if err != nil {
		return err
	}
	if err := s.repository.UpdateTree(ctx, item, current.TreePath, actorID); err != nil {
		return mapConflict(err, "部门编码已存在")
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, id, actorID int64) error {
	if id <= 0 {
		return apperror.InvalidArgument("A0400", "部门ID无效", nil)
	}
	children, err := s.repository.HasChildren(ctx, id)
	if err != nil {
		return apperror.Internal(err)
	}
	accounts, err := s.repository.HasAccounts(ctx, id)
	if err != nil {
		return apperror.Internal(err)
	}
	if children || accounts {
		return apperror.Conflict("A0409", "部门存在下级部门或用户，不能删除")
	}
	if err := s.repository.Delete(ctx, id, actorID); err != nil {
		return mapNotFound(err, "部门不存在")
	}
	return nil
}

func (s *Service) prepare(ctx context.Context, command SaveCommand) (domain.Department, error) {
	item := domain.Department{ID: command.ID, Name: strings.TrimSpace(command.Name), Code: strings.TrimSpace(command.Code), ParentID: command.ParentID, Sort: command.Sort, Status: command.Status}
	if err := item.Validate(); err != nil {
		return domain.Department{}, apperror.InvalidArgument("A0400", "部门名称、编码或状态无效", err)
	}
	codeExists, err := s.repository.CodeExists(ctx, item.Code, item.ID)
	if err != nil {
		return domain.Department{}, apperror.Internal(err)
	}
	nameExists, err := s.repository.NameExists(ctx, item.Name, item.ParentID, item.ID)
	if err != nil {
		return domain.Department{}, apperror.Internal(err)
	}
	if codeExists || nameExists {
		return domain.Department{}, apperror.Conflict("A0409", "部门名称或编码已存在")
	}
	item.TreePath = "0"
	if item.ParentID != 0 {
		parent, err := s.repository.Get(ctx, item.ParentID)
		if err != nil {
			return domain.Department{}, mapNotFound(err, "上级部门不存在")
		}
		item.TreePath = fmt.Sprintf("%s,%d", parent.TreePath, parent.ID)
	}
	return item, nil
}

func intPointer(value int) *int { return &value }

func mapNotFound(err error, message string) error {
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

var ErrNotFound = errors.New("not found")
