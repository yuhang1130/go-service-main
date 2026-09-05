package application

import (
	"context"
	"testing"
	"time"

	"github.com/yuhang1130/go-service-main/internal/features/notice/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/apperror"
)

type repositoryStub struct{ Repository }

func (repositoryStub) Create(context.Context, domain.Notice, int64) (int64, error) { return 42, nil }

type sanitizerStub struct{}

func (sanitizerStub) Sanitize(value string) string { return value }

type publisherStub struct{ published []PublishedNotice }

func (p *publisherStub) PublishNotice(_ context.Context, event PublishedNotice) {
	p.published = append(p.published, event)
}
func (*publisherStub) RevokeNotice(context.Context, RevokedNotice) {}

func TestPublishedCreateUsesPersistedIDInEvent(t *testing.T) {
	publisher := &publisherStub{}
	service := NewService(repositoryStub{}, sanitizerStub{}, publisher)
	err := service.Save(context.Background(), Command{Title: "通知", Content: "内容", Type: 1, Level: "L", Status: domain.StatusPublished, TargetType: domain.TargetAll}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(publisher.published) != 1 || publisher.published[0].ID != 42 {
		t.Fatalf("published events = %#v", publisher.published)
	}
}

type stateRepositoryStub struct {
	Repository
	item         domain.Notice
	publishCalls int
	revokeCalls  int
	deleteCalls  int
}

func (r *stateRepositoryStub) Get(context.Context, int64) (domain.Notice, error) {
	return r.item, nil
}

func (r *stateRepositoryStub) Publish(context.Context, int64, int64, time.Time) (domain.Notice, error) {
	r.publishCalls++
	if r.item.PublishStatus != domain.StatusDraft {
		return domain.Notice{}, domain.ErrInvalidTransition
	}
	return r.item, nil
}

func (r *stateRepositoryStub) Revoke(context.Context, int64, int64, time.Time) (domain.Notice, error) {
	r.revokeCalls++
	if r.item.PublishStatus != domain.StatusPublished {
		return domain.Notice{}, domain.ErrInvalidTransition
	}
	return r.item, nil
}

func (r *stateRepositoryStub) Delete(context.Context, []int64, int64) error {
	r.deleteCalls++
	return nil
}

func TestPublishedNoticeCannotBePublishedOrEditedAgain(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	repository := &stateRepositoryStub{item: domain.Notice{ID: 9, Title: "通知", Content: "内容", Type: 1, Level: "L", TargetType: domain.TargetAll, PublishStatus: domain.StatusPublished, PublisherID: 1, PublishTime: &now}}
	service := NewService(repository, sanitizerStub{}, nil)

	if err := service.Publish(context.Background(), 9, 2); apperror.As(err).Code != "A0409" {
		t.Fatalf("Publish() error = %v, want A0409", err)
	}
	if err := service.Save(context.Background(), Command{ID: 9, Title: "修改", Content: "内容", Type: 1, Level: "L", Status: domain.StatusDraft, TargetType: domain.TargetAll}, 2); apperror.As(err).Code != "A0409" {
		t.Fatalf("Save() error = %v, want A0409", err)
	}
	if repository.publishCalls != 1 {
		t.Fatalf("publish calls = %d, want 1 state-checked repository call", repository.publishCalls)
	}
}

func TestDraftNoticeCannotBeRevoked(t *testing.T) {
	t.Parallel()
	repository := &stateRepositoryStub{item: domain.Notice{ID: 9, PublishStatus: domain.StatusDraft}}
	service := NewService(repository, sanitizerStub{}, nil)

	if err := service.Revoke(context.Background(), 9, 2); apperror.As(err).Code != "A0409" {
		t.Fatalf("Revoke() error = %v, want A0409", err)
	}
	if repository.revokeCalls != 1 {
		t.Fatalf("revoke calls = %d, want 1 state-checked repository call", repository.revokeCalls)
	}
}
