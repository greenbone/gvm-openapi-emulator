package openapi

import (
	"github.com/getkin/kin-openapi/openapi3"
	"net/http"
)

type IRouterProvider interface {
	FindRoute(method, path string) *Route
	GetRoutes() []Route
}

type ISpecProvider interface {
	TryGetExampleBody(swaggerPath, method string) ([]byte, bool)
	FindOperation(swaggerPath, method string) *openapi3.Operation
	GetSpec() *Spec
}

type IValidator interface {
	HasRequiredBodyParam(swaggerPath, method string) bool
	IsEmptyBody(r *http.Request) (bool, error)
}
