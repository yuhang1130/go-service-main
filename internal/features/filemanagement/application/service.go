package application

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yuhang1130/go-service-main/internal/features/filemanagement/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/apperror"
)

var ErrNotFound = errors.New("not found")

type Storage interface {
	Put(context.Context, string, io.Reader, int64, int64, string) (int64, error)
	Open(context.Context, string) (io.ReadCloser, int64, error)
	Delete(context.Context, string) error
}

type Service struct {
	storage Storage
	maximum int64
}

func NewService(storage Storage, maximum int64) *Service {
	return &Service{storage: storage, maximum: maximum}
}

func (s *Service) Upload(ctx context.Context, name string, size int64, content io.Reader) (domain.File, error) {
	extension, err := domain.ValidateUpload(name, size, s.maximum)
	if err != nil {
		return domain.File{}, apperror.InvalidArgument("A0400", "文件类型或大小无效", err)
	}
	prefix := make([]byte, 512)
	read, readErr := io.ReadFull(content, prefix)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return domain.File{}, apperror.InvalidArgument("A0400", "文件内容无效", readErr)
	}
	if read == 0 {
		return domain.File{}, apperror.InvalidArgument("A0400", "文件内容为空", nil)
	}
	detectedType := strings.TrimSpace(strings.Split(http.DetectContentType(prefix[:read]), ";")[0])
	if err := domain.ValidateContentType(extension, detectedType); err != nil {
		return domain.File{}, apperror.InvalidArgument("A0400", "文件内容与扩展名不匹配", err)
	}
	content = io.MultiReader(bytes.NewReader(prefix[:read]), content)
	key := time.Now().UTC().Format("2006/01") + "/" + uuid.NewString() + extension
	contentType := domain.ContentType(extension)
	written, err := s.storage.Put(ctx, key, content, size, s.maximum, contentType)
	if err != nil {
		if errors.Is(err, ErrTooLarge) {
			return domain.File{}, apperror.InvalidArgument("A0400", "文件超过大小限制", nil)
		}
		return domain.File{}, apperror.Internal(err)
	}
	return domain.File{Key: key, Name: path.Base(name), Size: written, ContentType: contentType}, nil
}

func (s *Service) UploadImage(ctx context.Context, name string, size int64, content io.Reader) (domain.File, error) {
	if err := domain.ValidateImage(name); err != nil {
		return domain.File{}, apperror.InvalidArgument("A0400", "图片格式无效", err)
	}
	return s.Upload(ctx, name, size, content)
}

func (s *Service) Open(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	key, err := validateKey(key)
	if err != nil {
		return nil, 0, apperror.NotFound("A0404", "文件不存在")
	}
	reader, size, err := s.storage.Open(ctx, key)
	if errors.Is(err, ErrNotFound) {
		return nil, 0, apperror.NotFound("A0404", "文件不存在")
	}
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	return reader, size, nil
}

func (s *Service) Delete(ctx context.Context, raw string) error {
	key, err := keyFromPath(raw)
	if err != nil {
		return apperror.InvalidArgument("A0400", "文件路径无效", err)
	}
	if err := s.storage.Delete(ctx, key); err != nil {
		if errors.Is(err, ErrNotFound) {
			return apperror.NotFound("A0404", "文件不存在")
		}
		return apperror.Internal(err)
	}
	return nil
}

var ErrTooLarge = errors.New("file is too large")

func keyFromPath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if index := strings.Index(raw, "/api/v1/files/content/"); index >= 0 {
		raw = raw[index+len("/api/v1/files/content/"):]
	}
	return validateKey(raw)
}

func validateKey(raw string) (string, error) {
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "/")
	cleaned := path.Clean(raw)
	if cleaned == "." || cleaned == "" || cleaned != raw || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "\\") {
		return "", errors.New("invalid storage key")
	}
	return cleaned, nil
}
