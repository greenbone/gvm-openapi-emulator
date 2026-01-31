package openapi

import (
	"regexp"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi3"
)

type Route struct {
	Method     string
	Swagger    string
	Regex      *regexp.Regexp
	SampleFile string
}

type Spec struct {
	Doc3 *openapi3.T
	Doc2 *openapi2.T
}

type versionProbe struct {
	Swagger string `json:"swagger"`
	OpenAPI string `json:"openapi"`
}
