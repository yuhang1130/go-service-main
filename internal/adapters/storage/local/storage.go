package local

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	fileapp "github.com/yuhang1130/go-service-main/internal/features/filemanagement/application"
)

type Storage struct{ root string }

func New(root string) (*Storage, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(absolute, 0o750); err != nil {
		return nil, err
	}
	return &Storage{root: absolute}, nil
}

func (s *Storage) Put(ctx context.Context, key string, content io.Reader, _ int64, maximum int64, _ string) (int64, error) {
	target, err := s.resolve(key)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return 0, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(target), ".upload-*")
	if err != nil {
		return 0, err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	written, copyErr := io.Copy(temporary, io.LimitReader(content, maximum+1))
	closeErr := temporary.Close()
	if copyErr != nil {
		return 0, copyErr
	}
	if closeErr != nil {
		return 0, closeErr
	}
	if written > maximum {
		return 0, fileapp.ErrTooLarge
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if err := os.Chmod(temporaryName, 0o640); err != nil {
		return 0, err
	}
	if err := os.Rename(temporaryName, target); err != nil {
		return 0, err
	}
	return written, nil
}

func (s *Storage) Open(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	target, err := s.resolve(key)
	if err != nil {
		return nil, 0, err
	}
	file, err := os.Open(target)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, fileapp.ErrNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, err
	}
	if !info.Mode().IsRegular() {
		file.Close()
		return nil, 0, fileapp.ErrNotFound
	}
	return file, info.Size(), nil
}

func (s *Storage) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	target, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(target); errors.Is(err, os.ErrNotExist) {
		return fileapp.ErrNotFound
	} else {
		return err
	}
}

func (s *Storage) resolve(key string) (string, error) {
	target := filepath.Join(s.root, filepath.FromSlash(key))
	relative, err := filepath.Rel(s.root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("storage key escapes root")
	}
	return target, nil
}
