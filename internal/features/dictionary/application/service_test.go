package application

import (
	"context"
	"testing"

	"github.com/yuhang1130/go-service-main/internal/features/dictionary/domain"
)

type repositoryStub struct{ Repository }

func (repositoryStub) CodeExists(context.Context, string, int64) (bool, error) { return false, nil }
func (repositoryStub) Create(context.Context, domain.Dictionary, int64) error  { return nil }

type publisherStub struct{ codes []string }

func (p *publisherStub) PublishDictionaryChanged(_ context.Context, code string) {
	p.codes = append(p.codes, code)
}

func TestSavePublishesDictionaryChangeAfterPersistence(t *testing.T) {
	publisher := &publisherStub{}
	service := NewService(repositoryStub{}, publisher)
	err := service.Save(context.Background(), DictionaryCommand{Code: "gender", Name: "性别", Status: 1}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(publisher.codes) != 1 || publisher.codes[0] != "gender" {
		t.Fatalf("published codes = %v", publisher.codes)
	}
}
