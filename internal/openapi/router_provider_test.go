package openapi

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestSwaggerPathToRegex_Static(t *testing.T) {
	re := swaggerPathToRegex("/v1/health")

	ok := []string{"/v1/health", "/v1/health/"}
	for _, p := range ok {
		if !re.MatchString(p) {
			t.Fatalf("expected %q to match %q", re.String(), p)
		}
	}

	bad := []string{"/v1/health/x", "/v1/healt", "/v1/health//"}
	for _, p := range bad {
		if re.MatchString(p) {
			t.Fatalf("expected %q to NOT match %q", re.String(), p)
		}
	}
}

func TestSwaggerPathToRegex_Param(t *testing.T) {
	re := swaggerPathToRegex("/users/{id}")

	if !re.MatchString("/users/123") {
		t.Fatalf("expected match")
	}
	if re.MatchString("/users/123/profile") {
		t.Fatalf("expected no match")
	}
}

func TestSwaggerPathToSampleName(t *testing.T) {
	got := swaggerPathToSampleName("get", "/users/{id}")
	want := "GET__users_{id}.json"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRouterProvider_FindRoute(t *testing.T) {
	p := &RouterProvider{
		routes: []Route{
			{
				Method:     "GET",
				Swagger:    "/users/{id}",
				Regex:      swaggerPathToRegex("/users/{id}"),
				SampleFile: "GET__users_{id}.json",
			},
			{
				Method:     "POST",
				Swagger:    "/users",
				Regex:      swaggerPathToRegex("/users"),
				SampleFile: "POST__users.json",
			},
		},
	}

	r := p.FindRoute("get", "/users/55")
	if r == nil || r.Swagger != "/users/{id}" {
		t.Fatalf("expected to find /users/{id}, got %#v", r)
	}

	if p.FindRoute("GET", "/nope") != nil {
		t.Fatalf("expected nil")
	}
}

func TestNewRouterProvider_BuildRoutes(t *testing.T) {
	paths := openapi3.NewPaths()
	paths.Set("/users/{id}", &openapi3.PathItem{
		Get: &openapi3.Operation{Responses: openapi3.NewResponses()},
	})
	paths.Set("/users", &openapi3.PathItem{
		Post: &openapi3.Operation{Responses: openapi3.NewResponses()},
	})

	doc := &openapi3.T{Paths: paths}
	spec := &Spec{Doc3: doc}

	provider := NewRouterProvider(spec)
	if provider == nil {
		t.Fatalf("expected provider, got nil")
	}

	rp, ok := provider.(*RouterProvider)
	if !ok {
		t.Fatalf("expected *RouterProvider, got %T", provider)
	}

	var foundGet, foundPost bool
	for _, r := range rp.routes {
		if r.Method == "GET" && r.Swagger == "/users/{id}" {
			foundGet = true
			if r.SampleFile != "GET__users_{id}.json" {
				t.Fatalf("bad sample file: %q", r.SampleFile)
			}
			if !r.Regex.MatchString("/users/1") {
				t.Fatalf("regex should match")
			}
		}
		if r.Method == "POST" && r.Swagger == "/users" {
			foundPost = true
			if r.SampleFile != "POST__users.json" {
				t.Fatalf("bad sample file: %q", r.SampleFile)
			}
			if !r.Regex.MatchString("/users") {
				t.Fatalf("regex should match")
			}
		}
	}

	if !foundGet || !foundPost {
		t.Fatalf("missing routes: foundGet=%v foundPost=%v routes=%#v", foundGet, foundPost, rp.routes)
	}
}

func TestNewRouterProvider_NilGuards(t *testing.T) {
	if got := NewRouterProvider(nil); got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}

	if got := NewRouterProvider(&Spec{Doc3: nil}); got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}

	if got := NewRouterProvider(&Spec{Doc3: &openapi3.T{}}); got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}
}

func TestRouterProvider_GetRoutes_ReturnsRoutes(t *testing.T) {
	p := &RouterProvider{
		routes: []Route{
			{Method: "GET", Swagger: "/x", Regex: swaggerPathToRegex("/x"), SampleFile: "GET__x.json"},
			{Method: "POST", Swagger: "/y", Regex: swaggerPathToRegex("/y"), SampleFile: "POST__y.json"},
		},
	}

	got := p.GetRoutes()
	if len(got) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(got))
	}
	if got[0].Swagger != "/x" || got[1].Swagger != "/y" {
		t.Fatalf("unexpected routes: %#v", got)
	}
}
