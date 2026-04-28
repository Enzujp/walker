package registry

import "github.com/enzujp/walker/internal"

var Routes []internal.RouteMeta

func Register(route internal.RouteMeta) {
	Routes = append(Routes, route)
}
