package api

import v1routes "card-arena/api/internal/api/v1"

// apiRoutes returns the already-bound endpoint routes. Each route owns its
// path metadata and handler; this file only exposes the complete v1 registry
// to the router.
func apiRoutes() []Route {
	return v1routes.Definitions()
}
