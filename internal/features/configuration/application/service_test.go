package application

import (
	"context"
	"errors"
	"testing"

	"github.com/yuhang1130/go-service-main/internal/features/configuration/domain"
)

type configurationRepositoryStub struct {
	Repository
	item    domain.Config
	created int
}

func (r *configurationRepositoryStub) GetByKey(context.Context, string) (domain.Config, error) {
	return r.item, nil
}

func (r *configurationRepositoryStub) KeyExists(context.Context, string, int64) (bool, error) {
	return false, nil
}

func (r *configurationRepositoryStub) Create(context.Context, domain.Config, int64) error {
	r.created++
	return nil
}

type configurationCacheStub struct {
	Cache
	item          domain.Config
	found         bool
	getErr        error
	invalidateErr error
	invalidations int
}

func (c *configurationCacheStub) Get(context.Context, string) (domain.Config, bool, uint64, error) {
	return c.item, c.found, 1, c.getErr
}

func (c *configurationCacheStub) Set(context.Context, domain.Config, uint64) error {
	return nil
}

func (c *configurationCacheStub) InvalidateAll(context.Context) error {
	c.invalidations++
	return c.invalidateErr
}

func TestGetByKeyFallsBackToDatabaseWhenCacheFails(t *testing.T) {
	t.Parallel()
	want := domain.Config{ID: 1, Name: "Feature", Key: "feature.enabled", Value: "true"}
	repository := &configurationRepositoryStub{item: want}
	cache := &configurationCacheStub{getErr: errors.New("redis unavailable")}
	service := NewService(repository, cache)

	got, err := service.GetByKey(context.Background(), want.Key)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("GetByKey() = %#v, want %#v", got, want)
	}
}

func TestSaveDoesNotTurnCommittedWriteIntoClientFailureWhenCacheInvalidationFails(t *testing.T) {
	t.Parallel()
	repository := &configurationRepositoryStub{}
	cache := &configurationCacheStub{invalidateErr: errors.New("redis unavailable")}
	service := NewService(repository, cache)

	err := service.Save(context.Background(), Command{Name: "Feature", Key: "feature.enabled", Value: "true"}, 1)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if repository.created != 1 || cache.invalidations != 1 {
		t.Fatalf("created = %d, invalidations = %d", repository.created, cache.invalidations)
	}
}
