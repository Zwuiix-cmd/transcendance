package api

import v1routes "card-arena/api/internal/api/v1"

// apiRoutes returns the already-bound endpoint routes: the v1 registry plus
// the realtime WebSocket endpoint owned by this package.
func apiRoutes() []Route {
	return append(v1routes.Definitions(), websocketDefinition)
}
