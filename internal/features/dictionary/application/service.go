package application

import (
	"context"
	"errors"
	"strings"

	"github.com/yuhang1130/go-service-main/internal/features/dictionary/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/apperror"
	"github.com/yuhang1130/go-service-main/internal/foundation/persistence"
)

var ErrNotFound = errors.New("not found")

type Query struct {
	Page, PageSize int
	Keywords       string
	Status         *int
}

type ItemQuery struct {
	Page, PageSize int
	DictCode       string
	Keywords       string
	Status         *int
}

type DictionaryCommand struct {
	ID     int64
	Code   string
	Name   string
	Status int
	Remark string
}

type ItemCommand struct {
	ID       int64
	DictCode string
	Value    string
	Label    string
	TagType  string
	Sort     int
	Status   int
	Remark   string
}

type Repository interface {
	List(context.Context, Query) ([]domain.Dictionary, int64, error)
	Options(context.Context) ([]domain.Dictionary, error)
	Get(context.Context, int64) (domain.Dictionary, error)
	DictionaryExists(context.Context, string) (bool, error)
	CodeExists(context.Context, string, int64) (bool, error)
	Create(context.Context, domain.Dictionary, int64) error
	Update(context.Context, domain.Dictionary, int64) error
	Delete(context.Context, int64, int64) error
	ItemCount(context.Context, string) (int64, error)
	ListItems(context.Context, ItemQuery) ([]domain.Item, int64, error)
	ItemOptions(context.Context, string) ([]domain.Item, error)
	GetItem(context.Context, int64, string) (domain.Item, error)
	ItemValueExists(context.Context, string, string, int64) (bool, error)
	CreateItem(context.Context, domain.Item, int64) error
	UpdateItem(context.Context, domain.Item, int64) error
	DeleteItems(context.Context, string, []int64) error
}

type ChangePublisher interface {
	PublishDictionaryChanged(context.Context, string)
}

type Service struct {
	repository Repository
	publisher  ChangePublisher
}

func NewService(repository Repository, publisher ChangePublisher) *Service {
	return &Service{repository: repository, publisher: publisher}
}

func (s *Service) List(ctx context.Context, query Query) ([]domain.Dictionary, int64, error) {
	items, total, err := s.repository.List(ctx, normalizeQuery(query))
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	return items, total, nil
}

func (s *Service) Options(ctx context.Context) ([]domain.Dictionary, error) {
	items, err := s.repository.Options(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return items, nil
}

func (s *Service) Get(ctx context.Context, id int64) (domain.Dictionary, error) {
	item, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.Dictionary{}, mapNotFound(err, "字典不存在")
	}
	return item, nil
}

func (s *Service) Save(ctx context.Context, command DictionaryCommand, actorID int64) error {
	item := domain.Dictionary{ID: command.ID, Code: strings.TrimSpace(command.Code), Name: strings.TrimSpace(command.Name), Status: command.Status, Remark: strings.TrimSpace(command.Remark)}
	if err := item.Validate(); err != nil {
		return apperror.InvalidArgument("A0400", "字典名称、编码或状态无效", err)
	}
	exists, err := s.repository.CodeExists(ctx, item.Code, item.ID)
	if err != nil {
		return apperror.Internal(err)
	}
	if exists {
		return apperror.Conflict("A0409", "字典编码已存在")
	}
	originalCode := ""
	if item.ID > 0 {
		original, getErr := s.repository.Get(ctx, item.ID)
		if getErr != nil {
			return mapNotFound(getErr, "字典不存在")
		}
		originalCode = original.Code
	}
	if item.ID == 0 {
		err = s.repository.Create(ctx, item, actorID)
	} else {
		err = s.repository.Update(ctx, item, actorID)
	}
	if err != nil {
		return mapMutation(err, "字典不存在", "字典编码已存在")
	}
	s.publishChanged(ctx, item.Code)
	if originalCode != "" && originalCode != item.Code {
		s.publishChanged(ctx, originalCode)
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, id, actorID int64) error {
	item, err := s.repository.Get(ctx, id)
	if err != nil {
		return mapNotFound(err, "字典不存在")
	}
	count, err := s.repository.ItemCount(ctx, item.Code)
	if err != nil {
		return apperror.Internal(err)
	}
	if count > 0 {
		return apperror.Conflict("A0409", "请先删除该字典下的所有字典项")
	}
	if err := s.repository.Delete(ctx, id, actorID); err != nil {
		return mapNotFound(err, "字典不存在")
	}
	s.publishChanged(ctx, item.Code)
	return nil
}

func (s *Service) ListItems(ctx context.Context, query ItemQuery) ([]domain.Item, int64, error) {
	query.Page, query.PageSize = normalizePage(query.Page, query.PageSize)
	query.DictCode, query.Keywords = strings.TrimSpace(query.DictCode), strings.TrimSpace(query.Keywords)
	items, total, err := s.repository.ListItems(ctx, query)
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	return items, total, nil
}

func (s *Service) ItemOptions(ctx context.Context, dictCode string) ([]domain.Item, error) {
	items, err := s.repository.ItemOptions(ctx, strings.TrimSpace(dictCode))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return items, nil
}

func (s *Service) GetItem(ctx context.Context, id int64, dictCode string) (domain.Item, error) {
	item, err := s.repository.GetItem(ctx, id, dictCode)
	if err != nil {
		return domain.Item{}, mapNotFound(err, "字典项不存在")
	}
	return item, nil
}

func (s *Service) SaveItem(ctx context.Context, command ItemCommand, actorID int64) error {
	item := domain.Item{ID: command.ID, DictCode: strings.TrimSpace(command.DictCode), Value: strings.TrimSpace(command.Value), Label: strings.TrimSpace(command.Label), TagType: strings.ToUpper(strings.TrimSpace(command.TagType)), Sort: command.Sort, Status: command.Status, Remark: strings.TrimSpace(command.Remark)}
	if item.TagType == "" {
		item.TagType = "N"
	}
	if err := item.Validate(); err != nil {
		return apperror.InvalidArgument("A0400", "字典项参数无效", err)
	}
	dictionaryExists, err := s.repository.DictionaryExists(ctx, item.DictCode)
	if err != nil {
		return apperror.Internal(err)
	}
	if !dictionaryExists {
		return apperror.NotFound("A0404", "字典不存在")
	}
	exists, err := s.repository.ItemValueExists(ctx, item.DictCode, item.Value, item.ID)
	if err != nil {
		return apperror.Internal(err)
	}
	if exists {
		return apperror.Conflict("A0409", "字典项值已存在")
	}
	if item.ID == 0 {
		err = s.repository.CreateItem(ctx, item, actorID)
	} else {
		err = s.repository.UpdateItem(ctx, item, actorID)
	}
	if err != nil {
		return mapMutation(err, "字典项不存在", "字典项值已存在")
	}
	s.publishChanged(ctx, item.DictCode)
	return nil
}

func (s *Service) DeleteItems(ctx context.Context, dictCode string, ids []int64) error {
	if strings.TrimSpace(dictCode) == "" || len(ids) == 0 {
		return apperror.InvalidArgument("A0400", "字典项ID无效", nil)
	}
	if err := s.repository.DeleteItems(ctx, dictCode, ids); err != nil {
		return mapNotFound(err, "字典项不存在")
	}
	s.publishChanged(ctx, dictCode)
	return nil
}

func (s *Service) publishChanged(ctx context.Context, dictCode string) {
	if s.publisher != nil {
		s.publisher.PublishDictionaryChanged(ctx, dictCode)
	}
}

func normalizeQuery(query Query) Query {
	query.Page, query.PageSize = normalizePage(query.Page, query.PageSize)
	query.Keywords = strings.TrimSpace(query.Keywords)
	return query
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

func mapNotFound(err error, message string) error {
	if errors.Is(err, ErrNotFound) {
		return apperror.NotFound("A0404", message)
	}
	return apperror.Internal(err)
}

func mapMutation(err error, notFoundMessage, conflictMessage string) error {
	if errors.Is(err, ErrNotFound) {
		return apperror.NotFound("A0404", notFoundMessage)
	}
	if errors.Is(err, persistence.ErrConflict) {
		return apperror.Conflict("A0409", conflictMessage)
	}
	return apperror.Internal(err)
}
