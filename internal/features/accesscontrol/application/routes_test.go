package application

import (
	"testing"

	"github.com/yuhang1130/go-service-main/internal/features/accesscontrol/domain"
)

func TestBuildRoutesExcludesButtons(t *testing.T) {
	routes := BuildRoutes([]domain.Menu{
		{ID: 1, ParentID: 0, Type: "C", Name: "System", Visible: 1},
		{ID: 2, ParentID: 1, Type: "M", Name: "Users", Visible: 1},
		{ID: 3, ParentID: 2, Type: "B", Name: "Create", Visible: 1},
	})
	if len(routes) != 1 || len(routes[0].Children) != 1 || len(routes[0].Children[0].Children) != 0 {
		t.Fatalf("unexpected routes: %#v", routes)
	}
}

func TestBuildRoutesIncludesHiddenMenusAsHiddenRoutes(t *testing.T) {
	routes := BuildRoutes([]domain.Menu{
		{ID: 1, ParentID: 0, Type: "C", Name: "System", Visible: 1},
		{ID: 2, ParentID: 1, Type: "M", Name: "Dictionary", RouteName: "Dict", Visible: 1},
		{ID: 3, ParentID: 1, Type: "M", Name: "Dictionary Items", RouteName: "DictItem", Visible: 0},
	})

	if len(routes) != 1 || len(routes[0].Children) != 2 {
		t.Fatalf("unexpected routes: %#v", routes)
	}
	hidden := routes[0].Children[1]
	if hidden.Name != "DictItem" || hidden.Meta == nil || !hidden.Meta.Hidden {
		t.Fatalf("hidden route = %#v, want DictItem with hidden metadata", hidden)
	}
}
