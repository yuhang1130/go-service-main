package adminapi

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yuhang1130/go-service-main/internal/foundation/auth"
)

type ID int64

type Flag int

func (id *ID) UnmarshalJSON(data []byte) error {
	var text string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
	} else {
		text = string(data)
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return err
	}
	*id = ID(value)
	return nil
}

func (flag *Flag) UnmarshalJSON(data []byte) error {
	var boolean bool
	if len(data) > 0 && (data[0] == 't' || data[0] == 'f') {
		if err := json.Unmarshal(data, &boolean); err != nil {
			return err
		}
		if boolean {
			*flag = 1
		} else {
			*flag = 0
		}
		return nil
	}
	value, err := strconv.Atoi(string(data))
	if err != nil || (value != 0 && value != 1) {
		return strconv.ErrSyntax
	}
	*flag = Flag(value)
	return nil
}

func AccountID(ctx *gin.Context) (int64, bool) {
	principal, ok := auth.PrincipalFrom(ctx.Request.Context())
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseInt(principal.Subject, 10, 64)
	return id, err == nil && id > 0
}

func ParseID(value string) (int64, error) { return strconv.ParseInt(value, 10, 64) }

func ParseIDs(value string) ([]int64, error) {
	parts := strings.Split(value, ",")
	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		id, err := ParseID(strings.TrimSpace(part))
		if err != nil || id <= 0 {
			return nil, strconv.ErrSyntax
		}
		ids = append(ids, id)
	}
	return ids, nil
}
