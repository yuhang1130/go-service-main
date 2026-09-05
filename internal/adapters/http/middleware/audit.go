package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/adminapi"
	auditdomain "github.com/yuhang1130/go-service-main/internal/features/audit/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/auth"
)

type AuditRecorder interface {
	Record(context.Context, auditdomain.Entry) error
}

func OperationAudit(recorder AuditRecorder, logger *slog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		ctx.Next()
		module, action, title, ok := classifyOperation(ctx.Request.Method, ctx.FullPath())
		if !ok {
			return
		}
		operatorID, _ := adminapi.AccountID(ctx)
		principal, _ := auth.PrincipalFrom(ctx.Request.Context())
		status := 1
		errorMessage := ""
		if ctx.Writer.Status() >= http.StatusBadRequest {
			status = 0
			errorMessage = "HTTP " + strconv.Itoa(ctx.Writer.Status())
		}
		userAgent := ctx.Request.UserAgent()
		entry := auditdomain.Entry{
			Module: module, ActionType: action, Title: title, OperatorID: operatorID, OperatorName: principal.Name,
			RequestURI: ctx.Request.URL.Path, RequestMethod: ctx.Request.Method, IP: ctx.ClientIP(),
			Device: device(userAgent), Browser: browser(userAgent), OS: operatingSystem(userAgent),
			Status: status, ErrorMessage: errorMessage, ExecutionTime: time.Since(start).Milliseconds(), CreateTime: time.Now().UTC(),
		}
		recordContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := recorder.Record(recordContext, entry); err != nil {
			logger.Warn("record operation audit failed", "error", err)
		}
	}
}

func classifyOperation(method, fullPath string) (string, string, string, bool) {
	path := strings.TrimPrefix(fullPath, "/api/v1/")
	segment := strings.Split(path, "/")[0]
	modules := map[string]string{
		"users": "用户管理", "roles": "角色管理", "menus": "菜单管理", "depts": "部门管理",
		"dicts": "字典管理", "configs": "参数配置", "notices": "通知公告", "logs": "操作日志", "files": "文件管理",
	}
	module, known := modules[segment]
	if !known {
		return "", "", "", false
	}
	action := ""
	switch method {
	case http.MethodPost:
		action = "新增"
	case http.MethodPut, http.MethodPatch:
		action = "修改"
	case http.MethodDelete:
		action = "删除"
	case http.MethodGet:
		if strings.HasSuffix(path, "/export") || strings.HasSuffix(path, "/template") {
			action = "导出"
		} else if path == segment {
			action = "查询"
		}
	}
	if strings.HasSuffix(path, "/import") {
		action = "导入"
	}
	if action == "" {
		return "", "", "", false
	}
	return module, action, module + "-" + action, true
}

func device(userAgent string) string {
	if strings.Contains(strings.ToLower(userAgent), "mobile") {
		return "移动端"
	}
	return "桌面端"
}

func browser(userAgent string) string {
	switch {
	case strings.Contains(userAgent, "Edg/"):
		return "Edge"
	case strings.Contains(userAgent, "Firefox/"):
		return "Firefox"
	case strings.Contains(userAgent, "Chrome/"):
		return "Chrome"
	case strings.Contains(userAgent, "Safari/"):
		return "Safari"
	default:
		return "其他"
	}
}

func operatingSystem(userAgent string) string {
	switch {
	case strings.Contains(userAgent, "Windows"):
		return "Windows"
	case strings.Contains(userAgent, "Mac OS X"):
		return "macOS"
	case strings.Contains(userAgent, "Android"):
		return "Android"
	case strings.Contains(userAgent, "iPhone"), strings.Contains(userAgent, "iPad"):
		return "iOS"
	case strings.Contains(userAgent, "Linux"):
		return "Linux"
	default:
		return "其他"
	}
}
