package application

import (
	"context"
	"io"
	"strings"
	"testing"
)

type storageStub struct {
	Storage
	key string
}

func (s *storageStub) Put(_ context.Context, key string, reader io.Reader, _, _ int64, _ string) (int64, error) {
	s.key = key
	data, err := io.ReadAll(reader)
	return int64(len(data)), err
}

func TestUploadUsesOpaqueDatePartitionedKey(t *testing.T) {
	t.Parallel()
	storage := &storageStub{}
	service := NewService(storage, 1024)
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	file, err := service.Upload(context.Background(), "avatar.png", int64(len(png)), strings.NewReader(string(png)))
	if err != nil {
		t.Fatal(err)
	}
	if file.Key != storage.key || strings.Contains(file.Key, "avatar") || !strings.HasSuffix(file.Key, ".png") {
		t.Fatalf("unexpected key %q", file.Key)
	}
}

func TestUploadRejectsContentThatDoesNotMatchExtension(t *testing.T) {
	t.Parallel()
	storage := &storageStub{}
	service := NewService(storage, 1024)
	content := "<html><script>alert(1)</script></html>"

	if _, err := service.Upload(context.Background(), "avatar.png", int64(len(content)), strings.NewReader(content)); err == nil {
		t.Fatal("Upload() error = nil, want content type mismatch")
	}
	if storage.key != "" {
		t.Fatalf("storage key = %q, want no write", storage.key)
	}
}
