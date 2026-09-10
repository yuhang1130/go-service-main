package redisstream

import (
	"time"

	"github.com/yuhang1130/go-service-main/internal/foundation/config"
)

func testConsumerConfig() config.TaskDispatch {
	return config.TaskDispatch{
		StreamPrefix: "test", Stream: "material:collection:v1",
		ConsumerGroup: "test-group", ReadBlock: time.Second,
		HandlerTimeout: time.Minute, ReclaimInterval: time.Second,
		ClaimMinIdle: time.Second, Concurrency: 1, MaxMessageBytes: 4096,
	}
}
