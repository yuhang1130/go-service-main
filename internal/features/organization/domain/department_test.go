package domain

import "testing"

func TestBuildTreeOrdersDepartmentsAndNestsChildren(t *testing.T) {
	items := []Department{{ID: 2, ParentID: 1, Sort: 1}, {ID: 1, ParentID: 0, Sort: 2}, {ID: 3, ParentID: 0, Sort: 1}}
	tree := BuildTree(items)
	if len(tree) != 2 || tree[0].ID != 3 || tree[1].ID != 1 || len(tree[1].Children) != 1 || tree[1].Children[0].ID != 2 {
		t.Fatalf("unexpected tree: %#v", tree)
	}
}
