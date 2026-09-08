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
