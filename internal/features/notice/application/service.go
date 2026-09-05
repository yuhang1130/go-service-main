package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/yuhang1130/go-service-main/internal/features/notice/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/apperror"
)

var ErrNotFound = errors.New("not found")

type Query struct {
	Page, PageSize int
	Title          string
	Type           *int
	Status         *int
	IsRead         *int
}

type Command struct {
	ID            int64
	Title         string
	Content       string
	Type          int
	Level         string
	Status        int
	TargetType    int
	TargetUserIDs []int64
}

type Repository interface {
	List(context.Context, Query) ([]domain.Notice, int64, error)
	ListMine(context.Context, int64, Query) ([]domain.Notice, int64, error)
	Get(context.Context, int64) (domain.Notice, error)
	GetVisible(context.Context, int64, int64) (domain.Notice, error)
	Create(context.Context, domain.Notice, int64) (int64, error)
	Update(context.Context, domain.Notice, int64) error
	Delete(context.Context, []int64, int64) error
	Publish(context.Context, int64, int64, time.Time) (domain.Notice, error)
	Revoke(context.Context, int64, int64, time.Time) (domain.Notice, error)
	MarkRead(context.Context, int64, int64, time.Time) error
	MarkAllRead(context.Context, int64, time.Time) error
	UnreadCount(context.Context, int64) (int64, error)
	AccountsExist(context.Context, []int64) (bool, error)
}

type EventPublisher interface {
	PublishNotice(context.Context, PublishedNotice)
	RevokeNotice(context.Context, RevokedNotice)
}

type PublishedNotice struct {
	ID            int64
	Title         string
	Type          int
	TargetType    int
	TargetUserIDs []int64
	PublishTime   time.Time
}

type RevokedNotice struct {
	ID            int64
	TargetType    int
	TargetUserIDs []int64
}

type ContentSanitizer interface {
	Sanitize(string) string
}

type Service struct {
	repository Repository
	sanitizer  ContentSanitizer
	publisher  EventPublisher
}

func NewService(repository Repository, sanitizer ContentSanitizer, publisher EventPublisher) *Service {
	return &Service{repository: repository, sanitizer: sanitizer, publisher: publisher}
}

func (s *Service) List(ctx context.Context, query Query) ([]domain.Notice, int64, error) {
	query = normalizeQuery(query)
	items, total, err := s.repository.List(ctx, query)
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	return items, total, nil
}

func (s *Service) ListMine(ctx context.Context, userID int64, query Query) ([]domain.Notice, int64, error) {
	query = normalizeQuery(query)
	items, total, err := s.repository.ListMine(ctx, userID, query)
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	return items, total, nil
}

func (s *Service) Get(ctx context.Context, id, userID int64, manager bool) (domain.Notice, error) {
	var item domain.Notice
	var err error
	if manager {
		item, err = s.repository.Get(ctx, id)
	} else {
		item, err = s.repository.GetVisible(ctx, id, userID)
	}
	if err != nil {
		return domain.Notice{}, mapNotFound(err)
	}
	if !manager {
		if err := s.repository.MarkRead(ctx, id, userID, time.Now().UTC()); err != nil {
			return domain.Notice{}, apperror.Internal(err)
		}
		item.IsRead = 1
	}
	return item, nil
}

func (s *Service) Save(ctx context.Context, command Command, actorID int64) error {
	if command.Status != domain.StatusDraft && command.Status != domain.StatusPublished {
		return apperror.InvalidArgument("A0400", "通知只能保存为草稿或直接发布", nil)
	}
	item := domain.Notice{ID: command.ID, Title: strings.TrimSpace(command.Title), Content: strings.TrimSpace(s.sanitizer.Sanitize(command.Content)), Type: command.Type, Level: strings.ToUpper(strings.TrimSpace(command.Level)), TargetType: command.TargetType, TargetUserIDs: uniquePositive(command.TargetUserIDs), PublishStatus: domain.StatusDraft}
	if item.TargetType == domain.TargetAll {
		item.TargetUserIDs = nil
	}
	if item.ID != 0 {
		current, err := s.repository.Get(ctx, item.ID)
		if err != nil {
			return mapNotFound(err)
		}
		if current.PublishStatus != domain.StatusDraft {
			return apperror.Conflict("A0409", "已发布或已撤回的通知不能编辑")
		}
	}
	if item.TargetType == domain.TargetSpecified {
		exists, err := s.repository.AccountsExist(ctx, item.TargetUserIDs)
		if err != nil {
			return apperror.Internal(err)
		}
		if !exists {
			return apperror.InvalidArgument("A0400", "通知目标用户无效", nil)
		}
	}
	now := time.Now().UTC()
	if command.Status == domain.StatusPublished {
		if err := item.Publish(actorID, now); err != nil {
			return apperror.InvalidArgument("A0400", "通知发布参数无效", err)
		}
	}
	if err := item.Validate(); err != nil {
		return apperror.InvalidArgument("A0400", "通知参数无效", err)
	}
	var err error
	if item.ID == 0 {
		item.ID, err = s.repository.Create(ctx, item, actorID)
	} else {
		err = s.repository.Update(ctx, item, actorID)
	}
	if err != nil {
		return mapMutationError(err)
	}
	if item.PublishStatus == domain.StatusPublished && item.PublishTime != nil {
		s.publishNotice(ctx, item)
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, ids []int64, actorID int64) error {
	if len(ids) == 0 {
		return apperror.InvalidArgument("A0400", "通知ID无效", nil)
	}
	if err := s.repository.Delete(ctx, ids, actorID); err != nil {
		if errors.Is(err, domain.ErrInvalidTransition) {
			return apperror.Conflict("A0409", "已发布通知请先撤回再删除")
		}
		return mapNotFound(err)
	}
	return nil
}

func (s *Service) Publish(ctx context.Context, id, actorID int64) error {
	now := time.Now().UTC()
	item, err := s.repository.Publish(ctx, id, actorID, now)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidTransition) {
			return apperror.Conflict("A0409", "只有草稿通知可以发布")
		}
		return mapMutationError(err)
	}
	s.publishNotice(ctx, item)
	return nil
}

func (s *Service) Revoke(ctx context.Context, id, actorID int64) error {
	now := time.Now().UTC()
	item, err := s.repository.Revoke(ctx, id, actorID, now)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidTransition) {
			return apperror.Conflict("A0409", "只有已发布通知可以撤回")
		}
		return mapMutationError(err)
	}
	if s.publisher != nil {
		s.publisher.RevokeNotice(ctx, RevokedNotice{ID: item.ID, TargetType: item.TargetType, TargetUserIDs: append([]int64(nil), item.TargetUserIDs...)})
	}
	return nil
}

func (s *Service) publishNotice(ctx context.Context, item domain.Notice) {
	if s.publisher == nil || item.PublishTime == nil {
		return
	}
	s.publisher.PublishNotice(ctx, PublishedNotice{ID: item.ID, Title: item.Title, Type: item.Type, TargetType: item.TargetType, TargetUserIDs: append([]int64(nil), item.TargetUserIDs...), PublishTime: *item.PublishTime})
}

func (s *Service) ReadAll(ctx context.Context, userID int64) error {
	if err := s.repository.MarkAllRead(ctx, userID, time.Now().UTC()); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

func (s *Service) UnreadCount(ctx context.Context, userID int64) (int64, error) {
	count, err := s.repository.UnreadCount(ctx, userID)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return count, nil
}

func normalizeQuery(query Query) Query {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 10
	}
	if query.PageSize > 200 {
		query.PageSize = 200
	}
	query.Title = strings.TrimSpace(query.Title)
	return query
}

func uniquePositive(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func mapNotFound(err error) error {
	if errors.Is(err, ErrNotFound) {
		return apperror.NotFound("A0404", "通知不存在")
	}
	return apperror.Internal(err)
}

func mapMutationError(err error) error {
	if errors.Is(err, domain.ErrInvalidTransition) {
		return apperror.Conflict("A0409", "通知状态已变化，请刷新后重试")
	}
	return mapNotFound(err)
}
