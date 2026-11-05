package route

import (
	"net/http"
	"strings"
)

// Access is the authorization boundary declared by an endpoint.
type Access uint8

const (
	AccessPublic Access = iota
	AccessOptionalUser
	AccessUser
	AccessAdmin
	AccessGameService
)

// Params contains values captured by named path parameters such as {id}.
type Params map[string]string

func (params Params) Get(name string) string {
	return params[name]
}

// Context is passed to every route handler and middleware.
type Context struct {
	Server  Runtime
	Writer  http.ResponseWriter
	Request *http.Request
	Params  Params
	User    any
}

func (context *Context) Param(name string) string {
	return context.Params.Get(name)
}

// Handler is the endpoint function owned by a route package.
type Handler func(*Context)

// Middleware wraps a route handler.
type Middleware func(*Context, Handler)

// RegisteredRoute is the polymorphic route contract consumed by the router.
type RegisteredRoute interface {
	Method() string
	Path() string
	Access() Access
	Middleware() []Middleware
	Handler() Handler
}

// Route is the reusable base route embedded by each concrete endpoint route.
// Embedding gives Go's structs method promotion similar to class inheritance;
// concrete routes override Handler by defining a method with the same name.
type Route struct {
	method     string
	path       string
	access     Access
	middleware []Middleware
}

func NewRoute(method, path string, access Access, middleware ...Middleware) Route {
	return Route{
		method:     strings.ToUpper(strings.TrimSpace(method)),
		path:       path,
		access:     access,
		middleware: append([]Middleware(nil), middleware...),
	}
}

func (route Route) Method() string {
	return route.method
}

func (route Route) Path() string {
	return route.path
}

func (route Route) Access() Access {
	return route.access
}

func (route Route) Middleware() []Middleware {
	return route.middleware
}

func (route Route) Handler() Handler {
	return NotImplemented
}

func NotImplemented(context *Context) {
	context.Writer.WriteHeader(http.StatusNotImplemented)
}

// Runtime is implemented by the API server. Route packages depend only on
// this contract, which keeps their handlers local without creating an import
// cycle back to the parent api package.
type Runtime interface {
	Authenticated(*http.Request) any
	RequireUser(http.ResponseWriter, *http.Request) any
	RequireAdmin(http.ResponseWriter, *http.Request) any
	RequireGameService(http.ResponseWriter, *http.Request) bool
}
