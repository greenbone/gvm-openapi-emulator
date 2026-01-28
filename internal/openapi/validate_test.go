package openapi

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestHasRequiredBodyParam_FalseWhenMissing(t *testing.T) {
	if HasRequiredBodyParam(nil, "/x", "post") {
		t.Fatalf("expected false")
	}

	if HasRequiredBodyParam(&Spec{Doc3: nil}, "/x", "post") {
		t.Fatalf("expected false")
	}

	if HasRequiredBodyParam(&Spec{Doc3: &openapi3.T{}}, "/missing", "post") {
		t.Fatalf("expected false")
	}
}

func TestHasRequiredBodyParam_FalseWhenNoRequestBody(t *testing.T) {
	paths := openapi3.NewPaths()
	paths.Set("/x", &openapi3.PathItem{
		Post: &openapi3.Operation{
			Responses: openapi3.NewResponses(),
		},
	})

	doc := &openapi3.T{}
	doc.Paths = paths
	spec := &Spec{Doc3: doc}

	if HasRequiredBodyParam(spec, "/x", "post") {
		t.Fatalf("expected false")
	}
}

func TestHasRequiredBodyParam_FalseWhenRequestBodyValueNil(t *testing.T) {
	paths := openapi3.NewPaths()
	paths.Set("/x", &openapi3.PathItem{
		Post: &openapi3.Operation{
			RequestBody: &openapi3.RequestBodyRef{},
			Responses:   openapi3.NewResponses(),
		},
	})

	doc := &openapi3.T{}
	doc.Paths = paths
	spec := &Spec{Doc3: doc}

	if HasRequiredBodyParam(spec, "/x", "post") {
		t.Fatalf("expected false")
	}
}

func TestHasRequiredBodyParam_TrueWhenRequired(t *testing.T) {
	paths := openapi3.NewPaths()
	paths.Set("/x", &openapi3.PathItem{
		Post: &openapi3.Operation{
			RequestBody: &openapi3.RequestBodyRef{
				Value: &openapi3.RequestBody{
					Required: true,
				},
			},
			Responses: openapi3.NewResponses(),
		},
	})

	doc := &openapi3.T{}
	doc.Paths = paths
	spec := &Spec{Doc3: doc}

	if !HasRequiredBodyParam(spec, "/x", "post") {
		t.Fatalf("expected true")
	}
}

func TestHasRequiredBodyParam_FalseWhenNotRequired(t *testing.T) {
	paths := openapi3.NewPaths()
	paths.Set("/x", &openapi3.PathItem{
		Post: &openapi3.Operation{
			RequestBody: &openapi3.RequestBodyRef{
				Value: &openapi3.RequestBody{
					Required: false,
				},
			},
			Responses: openapi3.NewResponses(),
		},
	})

	doc := &openapi3.T{}
	doc.Paths = paths
	spec := &Spec{Doc3: doc}

	if HasRequiredBodyParam(spec, "/x", "post") {
		t.Fatalf("expected false")
	}
}

func TestIsEmptyBody_NilBodyIsEmpty(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://example.com", nil)
	req.Body = nil

	empty, err := IsEmptyBody(req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !empty {
		t.Fatalf("expected empty")
	}
}

func TestIsEmptyBody_EmptyBytesIsEmptyAndRewindable(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://example.com",
		io.NopCloser(bytes.NewReader([]byte{})))

	empty, err := IsEmptyBody(req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !empty {
		t.Fatalf("expected empty")
	}

	b, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(b) != "" {
		t.Fatalf("expected body preserved, got %q", string(b))
	}
}

func TestIsEmptyBody_WhitespaceOnlyIsEmptyAndRewindable(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://example.com", io.NopCloser(stringsReader(" \n\t  ")))

	empty, err := IsEmptyBody(req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !empty {
		t.Fatalf("expected empty")
	}

	b, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(b) != " \n\t  " {
		t.Fatalf("expected body preserved, got %q", string(b))
	}
}

func TestIsEmptyBody_NonEmptyIsNotEmptyAndRewindable(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://example.com", io.NopCloser(stringsReader(`{"a":1}`)))

	empty, err := IsEmptyBody(req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if empty {
		t.Fatalf("expected not empty")
	}

	b, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(b) != `{"a":1}` {
		t.Fatalf("expected body preserved, got %q", string(b))
	}
}

type errReader struct{}

func (errReader) Read(p []byte) (int, error) { return 0, errors.New("boom") }
func (errReader) Close() error               { return nil }

func TestIsEmptyBody_ReadError(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://example.com", io.NopCloser(errReader{}))

	_, err := IsEmptyBody(req)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func stringsReader(s string) io.Reader { return strings.NewReader(s) }
