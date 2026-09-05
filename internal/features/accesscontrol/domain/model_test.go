package domain

import "testing"

func TestMergeScopesUsesUnionAndAllWins(t *testing.T) {
	merged := MergeScopes(9, []RoleScope{{Kind: ScopeSelf}, {Kind: ScopeCustom, DepartmentIDs: []int64{2, 3}}, {Kind: ScopeDepartment, DepartmentIDs: []int64{3, 4}}})
	if merged.All || merged.SelfID != 9 || len(merged.DepartmentIDs) != 3 {
		t.Fatalf("unexpected merged scope: %#v", merged)
	}
	all := MergeScopes(9, []RoleScope{{Kind: ScopeCustom, DepartmentIDs: []int64{2}}, {Kind: ScopeAll}})
	if !all.All {
		t.Fatalf("all-data scope must dominate: %#v", all)
	}
}

func TestBuildRoutesExcludesButtons(t *testing.T) {
	routes := BuildRoutes([]Menu{{ID: 1, ParentID: 0, Type: "C", Name: "System", Visible: 1}, {ID: 2, ParentID: 1, Type: "M", Name: "Users", Visible: 1}, {ID: 3, ParentID: 2, Type: "B", Name: "Create", Visible: 1}})
	if len(routes) != 1 || len(routes[0].Children) != 1 || len(routes[0].Children[0].Children) != 0 {
		t.Fatalf("unexpected routes: %#v", routes)
	}
}
