package v1

import (
	"encoding/json"
	"net/http"

	route "card-arena/api/internal/api/route"
)

type Route struct {
	route.Route
}

func Handle(context *route.Context) {
	context.Writer.Header().Set("Content-Type", "application/json")
	context.Writer.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(context.Writer).Encode(map[string]string{
		"service": "card-arena-api",
		"version": "v1",
	})
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodGet, "/api/v1", route.AccessPublic),
}
