package aliyunoss

import (
	"errors"
	"testing"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

func TestNotFoundMapping(t *testing.T) {
	err := oss.ServiceError{StatusCode: 404, Code: "NoSuchKey"}
	if !isNotFound(err) {
		t.Fatal("expected OSS not found error to be mapped")
	}
	if isNotFound(errors.New("network")) {
		t.Fatal("unexpected network error mapping")
	}
}
