package s3storage

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	fileapp "github.com/yuhang1130/go-service-main/internal/features/filemanagement/application"
)

type fakeClient struct {
	body []byte
}

func (f *fakeClient) PutObject(_ context.Context, input *awss3.PutObjectInput, _ ...func(*awss3.Options)) (*awss3.PutObjectOutput, error) {
	f.body, _ = io.ReadAll(input.Body)
	return &awss3.PutObjectOutput{}, nil
}
func (f *fakeClient) GetObject(_ context.Context, _ *awss3.GetObjectInput, _ ...func(*awss3.Options)) (*awss3.GetObjectOutput, error) {
	return &awss3.GetObjectOutput{Body: io.NopCloser(strings.NewReader(string(f.body))), ContentLength: aws.Int64(int64(len(f.body)))}, nil
}
func (f *fakeClient) HeadObject(_ context.Context, _ *awss3.HeadObjectInput, _ ...func(*awss3.Options)) (*awss3.HeadObjectOutput, error) {
	return &awss3.HeadObjectOutput{}, nil
}
func (f *fakeClient) DeleteObject(_ context.Context, _ *awss3.DeleteObjectInput, _ ...func(*awss3.Options)) (*awss3.DeleteObjectOutput, error) {
	return &awss3.DeleteObjectOutput{}, nil
}

func TestStorageRoundTrip(t *testing.T) {
	fake := &fakeClient{}
	storage := &Storage{client: fake, bucket: "bucket"}
	if _, err := storage.Put(context.Background(), "a.txt", strings.NewReader("hello"), 5, 10, "text/plain"); err != nil {
		t.Fatal(err)
	}
	reader, size, err := storage.Open(context.Background(), "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	value, _ := io.ReadAll(reader)
	if size != 5 || string(value) != "hello" {
		t.Fatalf("size=%d value=%q", size, value)
	}
	if _, err := storage.Put(context.Background(), "large.txt", strings.NewReader("hello"), 5, 4, "text/plain"); !errors.Is(err, fileapp.ErrTooLarge) {
		t.Fatalf("expected too large, got %v", err)
	}
}

func TestNotFoundMapping(t *testing.T) {
	err := &smithy.GenericAPIError{Code: "NoSuchKey", Message: "missing"}
	if !isNotFound(err) {
		t.Fatal("expected NoSuchKey to be mapped")
	}
}
