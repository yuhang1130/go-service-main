package domain

import (
	"errors"
	"strings"
	"time"
)

const (
	ScopeAll            = 1
	ScopeDepartmentTree = 2
	ScopeDepartment     = 3
	ScopeSelf           = 4
	ScopeCustom         = 5
	RootRoleCode        = "ROOT"
)

var ErrInvalid = errors.New("invalid access control value")

type Role struct {
	ID            int64
	Name          string
	Code          string
	Sort          int
	Status        int
	DataScope     int
	DepartmentIDs []int64
	MenuIDs       []int64
	CreateTime    time.Time
	UpdateTime    time.Time
}

func (r Role) Validate() error {
	if strings.TrimSpace(r.Name) == "" || strings.TrimSpace(r.Code) == "" || (r.Status != 0 && r.Status != 1) || r.DataScope < ScopeAll || r.DataScope > ScopeCustom {
		return ErrInvalid
	}
	return nil
}

type Menu struct {
	ID          int64
	ParentID    int64
	TreePath    string
	Name        string
	Type        string
	RouteName   string
	RoutePath   string
	Component   string
	ExternalURL string
	Permission  string
	AlwaysShow  int
	KeepAlive   int
	Visible     int
	Sort        int
	Icon        string
	Redirect    string
	Params      map[string]any
	CreateTime  time.Time
	UpdateTime  time.Time
	Children    []*Menu
}

func (m Menu) Validate() error {
	if strings.TrimSpace(m.Name) == "" || !strings.Contains("CMBE", m.Type) || len(m.Type) != 1 || m.ParentID < 0 || (m.Visible != 0 && m.Visible != 1) {
		return ErrInvalid
	}
	if (m.AlwaysShow != 0 && m.AlwaysShow != 1) || (m.KeepAlive != 0 && m.KeepAlive != 1) {
		return ErrInvalid
	}
	if m.Type == "B" && strings.TrimSpace(m.Permission) == "" {
		return ErrInvalid
	}
	if m.Type == "C" && strings.TrimSpace(m.RoutePath) == "" {
		return ErrInvalid
	}
	if m.Type == "M" && (strings.TrimSpace(m.RouteName) == "" || strings.TrimSpace(m.RoutePath) == "" || strings.TrimSpace(m.Component) == "") {
		return ErrInvalid
	}
	if m.Type == "E" && strings.TrimSpace(m.ExternalURL) == "" {
		return ErrInvalid
	}
	return nil
}

type Route struct {
	Path      string     `json:"path"`
	Name      string     `json:"name"`
	Component string     `json:"component"`
	Redirect  string     `json:"redirect,omitempty"`
	Meta      *RouteMeta `json:"meta,omitempty"`
	Children  []*Route   `json:"children,omitempty"`
}

type RouteMeta struct {
	Title       string         `json:"title"`
	Icon        string         `json:"icon,omitempty"`
	Hidden      bool           `json:"hidden,omitempty"`
	AlwaysShow  bool           `json:"alwaysShow,omitempty"`
	KeepAlive   bool           `json:"keepAlive,omitempty"`
	Params      map[string]any `json:"params,omitempty"`
	ExternalURL string         `json:"externalUrl,omitempty"`
}

type Authorization struct {
	Roles       []string
	Permissions []string
	System      bool
}

type AccountScope struct {
	All           bool
	SelfID        int64
	DepartmentIDs []int64
}

type RoleScope struct {
	Kind          int
	DepartmentIDs []int64
}

func MergeScopes(accountID int64, scopes []RoleScope) AccountScope {
	result := AccountScope{SelfID: accountID}
	seen := map[int64]struct{}{}
	for _, scope := range scopes {
		if scope.Kind == ScopeAll {
			return AccountScope{All: true, SelfID: accountID}
		}
		for _, id := range scope.DepartmentIDs {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				result.DepartmentIDs = append(result.DepartmentIDs, id)
			}
		}
	}
	return result
}

func BuildMenuTree(items []Menu) []*Menu {
	byID := make(map[int64]*Menu, len(items))
	for index := range items {
		item := items[index]
		item.Children = nil
		byID[item.ID] = &item
	}
	roots := make([]*Menu, 0)
	for _, item := range byID {
		if parent, ok := byID[item.ParentID]; ok && item.ParentID != item.ID {
			parent.Children = append(parent.Children, item)
		} else {
			roots = append(roots, item)
		}
	}
	sortMenus(roots)
	return roots
}

func BuildRoutes(items []Menu) []*Route { return buildRoutes(items, 0) }

func buildRoutes(items []Menu, parentID int64) []*Route {
	routes := make([]*Route, 0)
	for _, menu := range items {
		if menu.ParentID != parentID || menu.Type == "B" || menu.Visible == 0 {
			continue
		}
		isExternal := menu.Type == "E"
		isEmbedded := isExternal && menu.Component == "iframe"
		path, component := menu.RoutePath, menu.Component
		if isExternal && !isEmbedded && menu.ExternalURL != "" {
			path, component = menu.ExternalURL, ""
		}
		meta := &RouteMeta{Title: menu.Name, Icon: menu.Icon, Hidden: menu.Visible == 0, AlwaysShow: menu.AlwaysShow == 1, KeepAlive: (menu.Type == "M" || isEmbedded) && menu.KeepAlive == 1, Params: menu.Params}
		if isEmbedded {
			component, meta.ExternalURL = "iframe", menu.ExternalURL
		}
		route := &Route{Path: path, Name: menu.RouteName, Component: component, Redirect: menu.Redirect, Meta: meta, Children: buildRoutes(items, menu.ID)}
		if len(route.Children) == 0 {
			route.Children = nil
		}
		routes = append(routes, route)
	}
	return routes
}

func sortMenus(items []*Menu) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Sort < items[i].Sort || (items[j].Sort == items[i].Sort && items[j].ID < items[i].ID) {
				items[i], items[j] = items[j], items[i]
			}
		}
		sortMenus(items[i].Children)
	}
}
