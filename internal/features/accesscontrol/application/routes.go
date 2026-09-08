package application

import "github.com/yuhang1130/go-service-main/internal/features/accesscontrol/domain"

type Route struct {
	Path      string
	Name      string
	Component string
	Redirect  string
	Meta      *RouteMeta
	Children  []*Route
}

type RouteMeta struct {
	Title       string
	Icon        string
	Hidden      bool
	AlwaysShow  bool
	KeepAlive   bool
	Params      map[string]any
	ExternalURL string
}

func BuildRoutes(items []domain.Menu) []*Route {
	return buildRoutes(items, 0)
}

func buildRoutes(items []domain.Menu, parentID int64) []*Route {
	routes := make([]*Route, 0)
	for _, menu := range items {
		if menu.ParentID != parentID || menu.Type == "B" {
			continue
		}
		isExternal := menu.Type == "E"
		isEmbedded := isExternal && menu.Component == "iframe"
		path, component := menu.RoutePath, menu.Component
		if isExternal && !isEmbedded && menu.ExternalURL != "" {
			path, component = menu.ExternalURL, ""
		}
		meta := &RouteMeta{
			Title: menu.Name, Icon: menu.Icon, Hidden: menu.Visible == 0,
			AlwaysShow: menu.AlwaysShow == 1,
			KeepAlive:  (menu.Type == "M" || isEmbedded) && menu.KeepAlive == 1,
			Params:     menu.Params,
		}
		if isEmbedded {
			component, meta.ExternalURL = "iframe", menu.ExternalURL
		}
		route := &Route{
			Path: path, Name: menu.RouteName, Component: component,
			Redirect: menu.Redirect, Meta: meta, Children: buildRoutes(items, menu.ID),
		}
		if len(route.Children) == 0 {
			route.Children = nil
		}
		routes = append(routes, route)
	}
	return routes
}
