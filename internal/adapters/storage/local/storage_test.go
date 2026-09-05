package local

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	fileapp "github.com/yuhang1130/go-service-main/internal/features/filemanagement/application"
)

func TestStorageRoundTripAndBoundary(t *testing.T) {
	t.Parallel()
	storage, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := storage.Put(ctx, "2026/09/test.txt", strings.NewReader("hello"), 5, 5, "text/plain"); err != nil {
		t.Fatal(err)
	}
	reader, size, err := storage.Open(ctx, "2026/09/test.txt")
	if err != nil {
		t.Fatal(err)
	}
	content, readErr := io.ReadAll(reader)
	reader.Close()
	if readErr != nil || size != 5 || string(content) != "hello" {
		t.Fatalf("content = %q, size = %d, err = %v", content, size, readErr)
	}
	if err := storage.Delete(ctx, "2026/09/test.txt"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := storage.Open(ctx, "2026/09/test.txt"); !errors.Is(err, fileapp.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	if _, err := storage.Put(ctx, "too-large.txt", strings.NewReader("123456"), 6, 5, "text/plain"); !errors.Is(err, fileapp.ErrTooLarge) {
		t.Fatalf("expected too large, got %v", err)
	}
	if _, err := storage.Put(ctx, "../../outside.txt", strings.NewReader("x"), 1, 5, "text/plain"); err == nil {
		t.Fatal("expected path traversal to be rejected")
	}
}
