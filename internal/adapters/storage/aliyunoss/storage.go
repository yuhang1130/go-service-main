package aliyunoss

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	fileapp "github.com/yuhang1130/go-service-main/internal/features/filemanagement/application"
	appconfig "github.com/yuhang1130/go-service-main/internal/foundation/config"
)

type Storage struct{ bucket *oss.Bucket }

func New(cfg appconfig.AliyunOSSStorage) (*Storage, error) {
	client, err := oss.New(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey)
	if err != nil {
		return nil, err
	}
	bucket, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return nil, err
	}
	return &Storage{bucket: bucket}, nil
}

func (s *Storage) Put(ctx context.Context, key string, content io.Reader, size, maximum int64, contentType string) (int64, error) {
	if size <= 0 || size > maximum {
		return 0, fileapp.ErrTooLarge
	}
	err := s.bucket.PutObject(key, io.LimitReader(content, maximum+1), oss.ContentLength(size), oss.ContentType(contentType), oss.WithContext(ctx))
	if err != nil {
		return 0, err
	}
	return size, nil
}

func (s *Storage) Open(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	metadata, err := s.bucket.GetObjectMeta(key, oss.WithContext(ctx))
	if isNotFound(err) {
		return nil, 0, fileapp.ErrNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	size, err := strconv.ParseInt(metadata.Get("Content-Length"), 10, 64)
	if err != nil {
		return nil, 0, err
	}
	reader, err := s.bucket.GetObject(key, oss.WithContext(ctx))
	if isNotFound(err) {
		return nil, 0, fileapp.ErrNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	return reader, size, nil
}

func (s *Storage) Delete(ctx context.Context, key string) error {
	exists, err := s.bucket.IsObjectExist(key, oss.WithContext(ctx))
	if isNotFound(err) || (err == nil && !exists) {
		return fileapp.ErrNotFound
	}
	if err != nil {
		return err
	}
	return s.bucket.DeleteObject(key, oss.WithContext(ctx))
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	var serviceError oss.ServiceError
	return errors.As(err, &serviceError) && (serviceError.StatusCode == http.StatusNotFound || serviceError.Code == "NoSuchKey" || serviceError.Code == "NoSuchObject")
}
