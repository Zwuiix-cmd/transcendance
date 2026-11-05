package api

import (
	"net/http"
	"sort"
	"strings"

	routecontract "card-arena/api/internal/api/route"
)

// RouteAccess describes the authentication boundary attached to a route.
// Keeping it in the route registry makes authorization visible next to the
// method and path instead of hiding it in nested dispatch functions.
type RouteAccess = routecontract.Access

const (
	AccessPublic       = routecontract.AccessPublic
	AccessOptionalUser = routecontract.AccessOptionalUser
	AccessUser         = routecontract.AccessUser
	AccessAdmin        = routecontract.AccessAdmin
	AccessGameService  = routecontract.AccessGameService
)

type Params = routecontract.Params
type Context = routecontract.Context
type Handler = routecontract.Handler
type Middleware = routecontract.Middleware
type Route = routecontract.RegisteredRoute

// Router matches requests and executes the declarative route registry.
type Router struct {
	server *Server
	routes []Route
}

type routeMatch struct {
	route  Route
	params Params
}

// NewRouter builds the API router and orders routes from the most specific
// path to the least specific one. The ordering means /users/search wins over
// /users/{id} without requiring special cases in request dispatching.
func NewRouter(server *Server) *Router {
	routes := apiRoutes()
	sort.SliceStable(routes, func(left, right int) bool {
		leftSpecificity := routeSpecificity(routes[left].Path())
		rightSpecificity := routeSpecificity(routes[right].Path())
		if leftSpecificity == rightSpecificity {
			return routeAccessPriority(routes[left].Access()) > routeAccessPriority(routes[right].Access())
		}
		return leftSpecificity > rightSpecificity
	})
	return &Router{server: server, routes: routes}
}

// Routes returns a copy of the registered routes. It is useful for diagnostics
// and keeps the registry inspectable without allowing callers to replace it.
func (router *Router) Routes() []Route {
	return append([]Route(nil), router.routes...)
}

// ServeHTTP implements http.Handler for all versioned API endpoints.
func (router *Router) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	match, pathFound, methodFound := router.findRoute(request.URL.Path, request.Method)
	if !pathFound {
		writeError(writer, http.StatusNotFound, "not_found", "Route was not found")
		return
	}

	context := &Context{
		Server:  router.server,
		Writer:  writer,
		Request: request,
		Params:  match.params,
	}
	if methodFound {
		router.execute(match.route, context, match.route.Handler())
		return
	}

	// Run the access middleware even for an unsupported method. This avoids
	// leaking protected route existence to unauthenticated callers and mirrors
	// the authorization boundary of the route they attempted to reach.
	router.execute(match.route, context, methodNotAllowed)
}

func (router *Router) findRoute(path, method string) (routeMatch, bool, bool) {
	method = strings.ToUpper(strings.TrimSpace(method))
	var fallback routeMatch
	fallbackPriority := -1
	pathFound := false

	for _, route := range router.routes {
		params, ok := matchRoute(route.Path(), path)
		if !ok {
			continue
		}
		pathFound = true
		if strings.EqualFold(route.Method(), method) {
			return routeMatch{route: route, params: params}, true, true
		}
		priority := routeAccessPriority(route.Access())
		if priority > fallbackPriority {
			fallback = routeMatch{route: route, params: params}
			fallbackPriority = priority
		}
	}
	return fallback, pathFound, false
}

func (router *Router) execute(route Route, context *Context, endpoint Handler) {
	if endpoint == nil {
		endpoint = route.Handler()
	}
	middlewares := make([]Middleware, 0, len(route.Middleware())+1)
	middlewares = append(middlewares, accessMiddleware(route.Access()))
	middlewares = append(middlewares, route.Middleware()...)
	chain(middlewares, endpoint)(context)
}

func methodNotAllowed(context *Context) {
	writeError(context.Writer, http.StatusMethodNotAllowed, "method_not_allowed", "Method is not allowed")
}

func accessMiddleware(access RouteAccess) Middleware {
	switch access {
	case AccessOptionalUser:
		return optionalUserAccess
	case AccessUser:
		return userAccess
	case AccessAdmin:
		return adminAccess
	case AccessGameService:
		return gameServiceAccess
	default:
		return publicAccess
	}
}

func publicAccess(context *Context, next Handler) {
	next(context)
}

func optionalUserAccess(context *Context, next Handler) {
	context.User = context.Server.Authenticated(context.Request)
	next(context)
}

func userAccess(context *Context, next Handler) {
	user := context.Server.RequireUser(context.Writer, context.Request)
	if user == nil {
		return
	}
	context.User = user
	next(context)
}

func adminAccess(context *Context, next Handler) {
	user := context.Server.RequireAdmin(context.Writer, context.Request)
	if user == nil {
		return
	}
	context.User = user
	next(context)
}

func gameServiceAccess(context *Context, next Handler) {
	if !context.Server.RequireGameService(context.Writer, context.Request) {
		return
	}
	next(context)
}

func chain(middlewares []Middleware, endpoint Handler) Handler {
	next := endpoint
	for index := len(middlewares) - 1; index >= 0; index-- {
		middleware := middlewares[index]
		previous := next
		next = func(context *Context) {
			middleware(context, previous)
		}
	}
	return next
}

func matchRoute(pattern, path string) (Params, bool) {
	patternParts := splitRoutePath(pattern)
	pathParts := splitRoutePath(path)
	if len(patternParts) != len(pathParts) {
		return nil, false
	}

	params := make(Params)
	for index, patternPart := range patternParts {
		pathPart := pathParts[index]
		if isRouteParameter(patternPart) {
			name := strings.TrimSuffix(strings.TrimPrefix(patternPart, "{"), "}")
			if name == "" {
				return nil, false
			}
			params[name] = pathPart
			continue
		}
		if patternPart != pathPart {
			return nil, false
		}
	}
	return params, true
}

func splitRoutePath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func isRouteParameter(part string) bool {
	return len(part) > 2 && strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}")
}

func routeSpecificity(path string) int {
	staticSegments := 0
	for _, part := range splitRoutePath(path) {
		if !isRouteParameter(part) {
			staticSegments++
		}
	}
	return staticSegments*100 + len(splitRoutePath(path))
}

func routeAccessPriority(access RouteAccess) int {
	switch access {
	case AccessGameService, AccessAdmin:
		return 3
	case AccessUser:
		return 2
	case AccessOptionalUser:
		return 1
	default:
		return 0
	}
}
