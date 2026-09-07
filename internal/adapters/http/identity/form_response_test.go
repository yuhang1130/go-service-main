package identity

import (
	"encoding/json"
	"reflect"
	"testing"

	identitydomain "github.com/yuhang1130/go-service-main/internal/features/identity/domain"
)

func TestAccountToFormSerializesRoleIDsAsStrings(t *testing.T) {
	payload, err := json.Marshal(accountToForm(identitydomain.Account{
		ID:           1,
		DepartmentID: 2,
		RoleIDs:      []int64{2, 9_007_199_254_740_993},
	}))
	if err != nil {
		t.Fatal(err)
	}

	var response struct {
		ID      string   `json:"id"`
		DeptID  string   `json:"deptId"`
		RoleIDs []string `json:"roleIds"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("decode account form: %v; payload = %s", err, payload)
	}
	if response.ID != "1" || response.DeptID != "2" || !reflect.DeepEqual(response.RoleIDs, []string{"2", "9007199254740993"}) {
		t.Fatalf("account form IDs = %#v, want string IDs", response)
	}
}
