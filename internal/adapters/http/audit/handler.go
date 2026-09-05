package audit

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/adminapi"
	"github.com/yuhang1130/go-service-main/internal/adapters/http/middleware"
	auditapp "github.com/yuhang1130/go-service-main/internal/features/audit/application"
	auditdomain "github.com/yuhang1130/go-service-main/internal/features/audit/domain"
)

type Handler struct{ service *auditapp.Service }

func NewHandler(service *auditapp.Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterProtected(router *gin.RouterGroup) {
	logs := router.Group("/logs", middleware.RequirePermission("sys:log:list"))
	logs.GET("/analytics/trend", h.trend)
	logs.GET("/analytics/overview", h.overview)
	logs.GET("", h.list)
}

func (h *Handler) list(ctx *gin.Context) {
	query := auditapp.Query{Page: queryInt(ctx, "pageNum", 1), PageSize: queryInt(ctx, "pageSize", 10), Keywords: ctx.Query("keywords")}
	created := ctx.QueryArray("createTime")
	if len(created) == 2 {
		from, fromErr := parseTimestamp(created[0], false)
		to, toErr := parseTimestamp(created[1], true)
		if fromErr != nil || toErr != nil {
			adminapi.Invalid(ctx, "操作时间范围无效")
			return
		}
		query.CreatedFrom, query.CreatedTo = &from, &to
	}
	items, total, err := h.service.List(ctx.Request.Context(), query)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	result := make([]gin.H, len(items))
	for index, item := range items {
		result[index] = response(item)
	}
	adminapi.Page(ctx, result, total)
}

func (h *Handler) trend(ctx *gin.Context) {
	start, startErr := time.ParseInLocation("2006-01-02", ctx.Query("startDate"), time.Local)
	end, endErr := time.ParseInLocation("2006-01-02", ctx.Query("endDate"), time.Local)
	if startErr != nil || endErr != nil {
		adminapi.Invalid(ctx, "日期格式无效")
		return
	}
	result, err := h.service.Trend(ctx.Request.Context(), start, end)
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, gin.H{"dates": result.Dates, "operationList": result.OperationList, "operatorList": result.OperatorList, "pvList": result.OperationList, "uvList": result.OperatorList})
}

func (h *Handler) overview(ctx *gin.Context) {
	result, err := h.service.Overview(ctx.Request.Context(), time.Now())
	if err != nil {
		adminapi.Error(ctx, err)
		return
	}
	adminapi.OK(ctx, gin.H{"todayOperatorCount": result.TodayOperatorCount, "totalOperatorCount": result.TotalOperatorCount, "operatorGrowthRate": result.OperatorGrowthRate, "todayOperationCount": result.TodayOperationCount, "totalOperationCount": result.TotalOperationCount, "operationGrowthRate": result.OperationGrowthRate, "todayUvCount": result.TodayOperatorCount, "totalUvCount": result.TotalOperatorCount, "uvGrowthRate": result.OperatorGrowthRate, "todayPvCount": result.TodayOperationCount, "totalPvCount": result.TotalOperationCount, "pvGrowthRate": result.OperationGrowthRate})
}

func response(item auditdomain.Entry) gin.H {
	return gin.H{"id": item.ID, "module": item.Module, "actionType": item.ActionType, "title": item.Title, "content": item.Content, "operatorId": item.OperatorID, "operatorName": item.OperatorName, "requestUri": item.RequestURI, "requestMethod": item.RequestMethod, "ip": item.IP, "region": item.Region, "device": item.Device, "browser": item.Browser, "os": item.OS, "status": item.Status, "executionTime": item.ExecutionTime, "errorMsg": item.ErrorMessage, "createTime": item.CreateTime.Local().Format("2006-01-02 15:04:05")}
}

func queryInt(ctx *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(ctx.Query(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func parseTimestamp(raw string, endOfDay bool) (time.Time, error) {
	if len(raw) == len("2006-01-02") {
		value, err := time.ParseInLocation("2006-01-02", raw, time.Local)
		if err == nil && endOfDay {
			value = value.Add(24*time.Hour - time.Nanosecond)
		}
		return value, err
	}
	return time.ParseInLocation("2006-01-02 15:04:05", raw, time.Local)
}
