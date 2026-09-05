package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	redisclient "github.com/redis/go-redis/v9"
	audithttp "github.com/yuhang1130/go-service-main/internal/adapters/http/audit"
	configurationhttp "github.com/yuhang1130/go-service-main/internal/adapters/http/configuration"
	dictionaryhttp "github.com/yuhang1130/go-service-main/internal/adapters/http/dictionary"
	filehttp "github.com/yuhang1130/go-service-main/internal/adapters/http/filemanagement"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/middleware"
	noticehttp "github.com/yuhang1130/go-service-main/internal/adapters/http/notice"
	auditmysql "github.com/yuhang1130/go-service-main/internal/adapters/mysql/audit"
	configurationmysql "github.com/yuhang1130/go-service-main/internal/adapters/mysql/configuration"
	dictionarymysql "github.com/yuhang1130/go-service-main/internal/adapters/mysql/dictionary"
	noticemysql "github.com/yuhang1130/go-service-main/internal/adapters/mysql/notice"
	sseadapter "github.com/yuhang1130/go-service-main/internal/adapters/realtime/sse"
	configurationredis "github.com/yuhang1130/go-service-main/internal/adapters/redis/configuration"
	"github.com/yuhang1130/go-service-main/internal/adapters/security/htmlsanitizer"
	aliyunstorage "github.com/yuhang1130/go-service-main/internal/adapters/storage/aliyunoss"
	localstorage "github.com/yuhang1130/go-service-main/internal/adapters/storage/local"
	s3storage "github.com/yuhang1130/go-service-main/internal/adapters/storage/s3"
	auditapp "github.com/yuhang1130/go-service-main/internal/features/audit/application"
	configurationapp "github.com/yuhang1130/go-service-main/internal/features/configuration/application"
	dictionaryapp "github.com/yuhang1130/go-service-main/internal/features/dictionary/application"
	fileapp "github.com/yuhang1130/go-service-main/internal/features/filemanagement/application"
	noticeapp "github.com/yuhang1130/go-service-main/internal/features/notice/application"
	"github.com/yuhang1130/go-service-main/internal/foundation/config"
	"gorm.io/gorm"
)

type administrationAPI struct {
	audit         middleware.AuditRecorder
	auditQueue    *middleware.AsyncAuditRecorder
	logger        *slog.Logger
	auditHTTP     *audithttp.Handler
	configuration *configurationhttp.Handler
	dictionary    *dictionaryhttp.Handler
	files         *filehttp.Handler
	notice        *noticehttp.Handler
	realtimeHTTP  *sseadapter.Handler
	realtimeHub   *sseadapter.Hub
	realtimeBus   *sseadapter.Bus
}

func wireAdministrationAPI(ctx context.Context, database *gorm.DB, redis *redisclient.Client, cfg config.FileStorage, logger *slog.Logger) (administrationAPI, error) {
	auditService := auditapp.NewService(auditmysql.NewRepository(database))
	realtimeHub := sseadapter.NewHub(logger)
	storage, err := wireFileStorage(ctx, cfg)
	if err != nil {
		return administrationAPI{}, err
	}
	realtimeBus, err := sseadapter.NewBus(ctx, redis, realtimeHub, logger)
	if err != nil {
		realtimeHub.Close()
		return administrationAPI{}, fmt.Errorf("subscribe SSE event bus: %w", err)
	}
	auditQueue := middleware.NewAsyncAuditRecorder(auditService, logger, 256, 2*time.Second)
	configurationService := configurationapp.NewService(configurationmysql.NewRepository(database), configurationredis.NewCache(redis))
	return administrationAPI{
		audit:         auditQueue,
		auditQueue:    auditQueue,
		logger:        logger,
		auditHTTP:     audithttp.NewHandler(auditService),
		configuration: configurationhttp.NewHandler(configurationService),
		dictionary:    dictionaryhttp.NewHandler(dictionaryapp.NewService(dictionarymysql.NewRepository(database), realtimeBus)),
		files:         filehttp.NewHandler(fileapp.NewService(storage, cfg.MaxFileBytes), cfg.PublicBaseURL),
		notice:        noticehttp.NewHandler(noticeapp.NewService(noticemysql.NewRepository(database), htmlsanitizer.New(), realtimeBus)),
		realtimeHTTP:  sseadapter.NewHandler(realtimeHub, realtimeBus),
		realtimeHub:   realtimeHub,
		realtimeBus:   realtimeBus,
	}, nil
}

func (a administrationAPI) close() {
	a.stopRealtime()
	if a.auditQueue != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.auditQueue.Close(ctx); err != nil && a.logger != nil {
			a.logger.Warn("drain operation audit queue failed", "error", err)
		}
	}
}

func (a administrationAPI) stopRealtime() {
	if a.realtimeHub != nil {
		a.realtimeHub.Close()
	}
	if a.realtimeBus != nil {
		a.realtimeBus.Close()
	}
}

func wireFileStorage(ctx context.Context, cfg config.FileStorage) (fileapp.Storage, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Type)) {
	case "local":
		return localstorage.New(cfg.Root)
	case "s3":
		return s3storage.New(ctx, cfg.S3)
	case "aliyun_oss":
		return aliyunstorage.New(cfg.AliyunOSS)
	default:
		return nil, fmt.Errorf("unsupported file storage type %q", cfg.Type)
	}
}
