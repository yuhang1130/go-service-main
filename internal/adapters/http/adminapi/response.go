package adminapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yuhang1130/go-service-main/internal/foundation/apperror"
)

const CodeSuccess = "00000"

type Result struct {
	Code      string `json:"code"`
	Msg       string `json:"msg"`
	Data      any    `json:"data"`
	RequestID string `json:"requestId,omitempty"`
}

func OK(ctx *gin.Context, data any) {
	ctx.JSON(http.StatusOK, Result{Code: CodeSuccess, Msg: "成功", Data: data})
}

func OKMessage(ctx *gin.Context, message string) {
	ctx.JSON(http.StatusOK, Result{Code: CodeSuccess, Msg: message})
}

func Page(ctx *gin.Context, list any, total int64) {
	OK(ctx, gin.H{"list": list, "total": total})
}

func Error(ctx *gin.Context, err error) {
	applicationError := apperror.As(err)
	ctx.JSON(applicationError.HTTPStatus, Result{
		Code: applicationError.Code, Msg: applicationError.Message, Data: nil,
		RequestID: ctx.GetString("request_id"),
	})
}

func Invalid(ctx *gin.Context, message string) {
	Error(ctx, apperror.InvalidArgument("A0400", message, nil))
}
