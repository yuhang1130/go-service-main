package s3storage

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	fileapp "github.com/yuhang1130/go-service-main/internal/features/filemanagement/application"
	appconfig "github.com/yuhang1130/go-service-main/internal/foundation/config"
)

type client interface {
	PutObject(context.Context, *awss3.PutObjectInput, ...func(*awss3.Options)) (*awss3.PutObjectOutput, error)
	GetObject(context.Context, *awss3.GetObjectInput, ...func(*awss3.Options)) (*awss3.GetObjectOutput, error)
	HeadObject(context.Context, *awss3.HeadObjectInput, ...func(*awss3.Options)) (*awss3.HeadObjectOutput, error)
	DeleteObject(context.Context, *awss3.DeleteObjectInput, ...func(*awss3.Options)) (*awss3.DeleteObjectOutput, error)
}

type Storage struct {
	client client
	bucket string
}

func New(ctx context.Context, cfg appconfig.S3Storage) (*Storage, error) {
	loadOptions := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(cfg.Region)}
	if strings.TrimSpace(cfg.AccessKey) != "" {
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, err
	}
	client := awss3.NewFromConfig(awsCfg, func(options *awss3.Options) {
		options.UsePathStyle = cfg.UsePathStyle
		if strings.TrimSpace(cfg.Endpoint) != "" {
			options.BaseEndpoint = aws.String(strings.TrimRight(cfg.Endpoint, "/"))
		}
	})
	return &Storage{client: client, bucket: cfg.Bucket}, nil
}

func (s *Storage) Put(ctx context.Context, key string, content io.Reader, size, maximum int64, contentType string) (int64, error) {
	if size <= 0 || size > maximum {
		return 0, fileapp.ErrTooLarge
	}
	_, err := s.client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          io.LimitReader(content, maximum+1),
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	if err != nil {
		return 0, err
	}
	return size, nil
}

func (s *Storage) Open(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	output, err := s.client.GetObject(ctx, &awss3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if isNotFound(err) {
		return nil, 0, fileapp.ErrNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	return output.Body, aws.ToInt64(output.ContentLength), nil
}

func (s *Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.HeadObject(ctx, &awss3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if isNotFound(err) {
		return fileapp.ErrNotFound
	}
	if err != nil {
		return err
	}
	_, err = s.client.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	return err
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	var apiError smithy.APIError
	if errors.As(err, &apiError) {
		switch apiError.ErrorCode() {
		case "NoSuchKey", "NotFound", "404":
			return true
		}
	}
	return false
}
