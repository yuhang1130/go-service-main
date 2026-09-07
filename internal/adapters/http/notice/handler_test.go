package notice

import (
	"reflect"
	"testing"

	noticedomain "github.com/yuhang1130/go-service-main/internal/features/notice/domain"
)

func TestNoticeResponseSerializesTargetIDsAsStrings(t *testing.T) {
	result := noticeResponse(noticedomain.Notice{ID: 7, TargetUserIDs: []int64{2, 9_007_199_254_740_993}})
	if result["id"] != "7" || !reflect.DeepEqual(result["targetUserIds"], []string{"2", "9007199254740993"}) {
		t.Fatalf("unexpected IDs: id=%#v targetUserIds=%#v", result["id"], result["targetUserIds"])
	}
}
