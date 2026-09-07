package audit

import (
	"testing"

	auditdomain "github.com/yuhang1130/go-service-main/internal/features/audit/domain"
)

func TestResponseSerializesIDsAsStrings(t *testing.T) {
	result := response(auditdomain.Entry{ID: 9_007_199_254_740_993, OperatorID: 42})
	if result["id"] != "9007199254740993" || result["operatorId"] != "42" {
		t.Fatalf("unexpected IDs: id=%#v operatorId=%#v", result["id"], result["operatorId"])
	}
}
