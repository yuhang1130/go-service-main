package application

import (
	"context"
	"errors"
	"strings"

	"github.com/yuhang1130/go-service-main/internal/features/configuration/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/apperror"
	"github.com/yuhang1130/go-service-main/internal/foundation/persistence"
)

var ErrNotFound = errors.New("not found")

type Query struct {
	Page, PageSize int
	Keywords       string
}

type Command struct {
	ID     int64
	Name   string
	Key    string
	Value  string
	Remark string
}

type Repository interface {
	List(context.Context, Query) ([]domain.Config, int64, error)
	Get(context.Context, int64) (domain.Config, error)
	GetByKey(context.Context, string) (domain.Config, error)
	KeyExists(context.Context, string, int64) (bool, error)
	Create(context.Context, domain.Config, int64) error
	Update(context.Context, domain.Config, int64) error
	Delete(context.Context, int64, int64) error
	DeleteMany(context.Context, []int64, int64) error
}

type Cache interface {
	Get(context.Context, string) (domain.Config, bool, uint64, error)
	Set(context.Context, domain.Config, uint64) error
	Invalidate(context.Context, string) error
	InvalidateAll(context.Context) error
}

func (s *Service) GetByKey(ctx context.Context, key string) (domain.Config, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.Config{}, apperror.InvalidArgument("A0400", "配置键无效", nil)
	}
	item, found, version, cacheErr := s.cache.Get(ctx, key)
	if cacheErr == nil && found {
		return item, nil
	}
	item, err := s.repository.GetByKey(ctx, key)
	if err != nil {
		return domain.Config{}, mapNotFound(err)
	}
	if cacheErr == nil {
		_ = s.cache.Set(ctx, item, version)
	}
	return item, nil
}

type Service struct {
	repository Repository
	cache      Cache
}

func NewService(repository Repository, cache Cache) *Service {
	return &Service{repository: repository, cache: cache}
}

func (s *Service) List(ctx context.Context, query Query) ([]domain.Config, int64, error) {
	query.Page, query.PageSize = normalizePage(query.Page, query.PageSize)
	query.Keywords = strings.TrimSpace(query.Keywords)
	items, total, err := s.repository.List(ctx, query)
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	return items, total, nil
}

func (s *Service) Get(ctx context.Context, id int64) (domain.Config, error) {
	item, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.Config{}, mapNotFound(err)
	}
	return item, nil
}

func (s *Service) Save(ctx context.Context, command Command, actorID int64) error {
	item := domain.Config{ID: command.ID, Name: strings.TrimSpace(command.Name), Key: strings.TrimSpace(command.Key), Value: command.Value, Remark: strings.TrimSpace(command.Remark)}
	if err := item.Validate(); err != nil {
		return apperror.InvalidArgument("A0400", "配置名称或配置键无效", err)
	}
	exists, err := s.repository.KeyExists(ctx, item.Key, item.ID)
	if err != nil {
		return apperror.Internal(err)
	}
	if exists {
		return apperror.Conflict("A0409", "配置键已存在")
	}
	if item.ID == 0 {
		err = s.repository.Create(ctx, item, actorID)
	} else {
		err = s.repository.Update(ctx, item, actorID)
	}
	if err != nil {
		return mapNotFound(err)
	}
	// The database mutation has committed. Cache eviction is best effort here so a
	// transient Redis failure cannot make the client retry an already-applied write.
	_ = s.cache.InvalidateAll(ctx)
	return nil
}

func (s *Service) Delete(ctx context.Context, id, actorID int64) error {
	item, err := s.repository.Get(ctx, id)
	if err != nil {
		return mapNotFound(err)
	}
	if err := s.repository.Delete(ctx, id, actorID); err != nil {
		return mapNotFound(err)
	}
	_ = s.cache.Invalidate(ctx, item.Key)
	return nil
}

func (s *Service) DeleteMany(ctx context.Context, ids []int64, actorID int64) error {
	if len(ids) == 0 {
		return apperror.InvalidArgument("A0400", "系统配置ID无效", nil)
	}
	if err := s.repository.DeleteMany(ctx, ids, actorID); err != nil {
		return mapNotFound(err)
	}
	_ = s.cache.InvalidateAll(ctx)
	return nil
}

func (s *Service) RefreshKey(ctx context.Context, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return apperror.InvalidArgument("A0400", "配置键无效", nil)
	}
	if err := s.cache.Invalidate(ctx, key); err != nil {
		return apperror.Internal(err)
	}
	_, err := s.GetByKey(ctx, key)
	return err
}

func (s *Service) Refresh(ctx context.Context) error {
	if err := s.cache.InvalidateAll(ctx); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

func mapNotFound(err error) error {
	if errors.Is(err, ErrNotFound) {
		return apperror.NotFound("A0404", "系统配置不存在")
	}
	if errors.Is(err, persistence.ErrConflict) {
		return apperror.Conflict("A0409", "配置键已存在")
	}
	return apperror.Internal(err)
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}
