package domain

import "testing"

func TestItemAcceptsOnlyStableTagCodes(t *testing.T) {
	t.Parallel()
	valid := Item{DictCode: "gender", Value: "1", Label: "男", TagType: "P", Status: 1}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	valid.TagType = "primary"
	if err := valid.Validate(); err == nil {
		t.Fatal("frontend label must be normalized to stable code before the domain boundary")
	}
}
