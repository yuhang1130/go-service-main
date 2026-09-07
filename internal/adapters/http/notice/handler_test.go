package notice

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	noticeapp "github.com/yuhang1130/go-service-main/internal/features/notice/application"
	noticedomain "github.com/yuhang1130/go-service-main/internal/features/notice/domain"
	"github.com/yuhang1130/go-service-main/internal/foundation/auth"
)

func TestNoticeResponseSerializesTargetIDsAsStrings(t *testing.T) {
	result := noticeResponse(noticedomain.Notice{ID: 7, TargetUserIDs: []int64{2, 9_007_199_254_740_993}})
	if result["id"] != "7" || !reflect.DeepEqual(result["targetUserIds"], []string{"2", "9007199254740993"}) {
		t.Fatalf("unexpected IDs: id=%#v targetUserIds=%#v", result["id"], result["targetUserIds"])
	}
}

func TestNoticeRequestAcceptsStringAndNumberTypes(t *testing.T) {
	for _, input := range []string{`{"type":"1"}`, `{"type":1}`} {
		var request noticeRequest
		if err := json.Unmarshal([]byte(input), &request); err != nil {
			t.Fatalf("unmarshal %s: %v", input, err)
		}
		if request.Type != 1 {
			t.Fatalf("type = %d, want 1", request.Type)
		}
	}
}

type detailRepositoryStub struct {
	noticeapp.Repository
	managerCalls int
	visibleCalls int
}

func (r *detailRepositoryStub) Get(context.Context, int64) (noticedomain.Notice, error) {
	r.managerCalls++
	return noticedomain.Notice{
		ID:            1,
		Title:         "草稿通知",
		Content:       "内容",
		Type:          1,
		Level:         "H",
		TargetType:    noticedomain.TargetAll,
		PublishStatus: noticedomain.StatusDraft,
	}, nil
}

func (r *detailRepositoryStub) GetVisible(context.Context, int64, int64) (noticedomain.Notice, error) {
	r.visibleCalls++
	return noticedomain.Notice{}, noticeapp.ErrNotFound
}

func TestNoticeDetailUsesManagerVisibilityForListPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name             string
		principal        auth.Principal
		wantStatus       int
		wantManagerCalls int
		wantVisibleCalls int
	}{
		{
			name:             "notice manager can view draft",
			principal:        auth.Principal{Subject: "42", Permissions: map[string]struct{}{"sys:notice:list": {}}},
			wantStatus:       http.StatusOK,
			wantManagerCalls: 1,
		},
		{
			name:             "ordinary user still uses visible notice scope",
			principal:        auth.Principal{Subject: "42", Permissions: map[string]struct{}{}},
			wantStatus:       http.StatusNotFound,
			wantVisibleCalls: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &detailRepositoryStub{}
			handler := NewHandler(noticeapp.NewService(repository, nil, nil))
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			request := httptest.NewRequest(http.MethodGet, "/api/v1/notices/1/detail", nil)
			ctx.Request = request.WithContext(auth.WithPrincipal(request.Context(), test.principal))
			ctx.Params = gin.Params{{Key: "id", Value: "1"}}

			handler.detail(ctx)

			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
			if repository.managerCalls != test.wantManagerCalls || repository.visibleCalls != test.wantVisibleCalls {
				t.Fatalf("repository calls = manager:%d visible:%d, want manager:%d visible:%d", repository.managerCalls, repository.visibleCalls, test.wantManagerCalls, test.wantVisibleCalls)
			}
		})
	}
}
