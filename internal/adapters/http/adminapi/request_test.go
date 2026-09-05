package adminapi

import (
	"encoding/json"
	"testing"
)

func TestIDAcceptsStringAndNumber(t *testing.T) {
	for _, input := range []string{`"42"`, `42`} {
		var id ID
		if err := json.Unmarshal([]byte(input), &id); err != nil {
			t.Fatalf("unmarshal %s: %v", input, err)
		}
		if id != 42 {
			t.Fatalf("id = %d, want 42", id)
		}
	}
}

func TestFlagAcceptsBooleanAndNumber(t *testing.T) {
	tests := map[string]Flag{"true": 1, "false": 0, "1": 1, "0": 0}
	for input, want := range tests {
		var flag Flag
		if err := json.Unmarshal([]byte(input), &flag); err != nil {
			t.Fatalf("unmarshal %s: %v", input, err)
		}
		if flag != want {
			t.Fatalf("flag = %d, want %d", flag, want)
		}
	}
}
