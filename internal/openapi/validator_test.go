// SPDX-FileCopyrightText: 2026 Greenbone AG
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package openapi

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/getkin/kin-openapi/openapi3"
)

type MockSpecProvider struct {
	mock.Mock
}

func (m *MockSpecProvider) TryGetExampleBody(swaggerPath, method string) ([]byte, bool) {
	args := m.Called(swaggerPath, method)
	b, _ := args.Get(0).([]byte)
	return b, args.Bool(1)
}

func (m *MockSpecProvider) FindOperation(swaggerPath, method string) *openapi3.Operation {
	args := m.Called(swaggerPath, method)
	op, _ := args.Get(0).(*openapi3.Operation)
	return op
}

func (m *MockSpecProvider) GetSpec() *Spec {
	args := m.Called()
	op, _ := args.Get(0).(*Spec)
	return op
}

type errReader struct{}

func (errReader) Read(p []byte) (int, error) { return 0, errors.New("boom") }
func (errReader) Close() error               { return nil }

func TestValidator_HasRequiredBodyParam_FalseWhenMissing(t *testing.T) {
	m := new(MockSpecProvider)
	v := NewValidator(m)

	m.On("FindOperation", "/x", "post").Return((*openapi3.Operation)(nil)).Once()

	require.False(t, v.HasRequiredBodyParam("/x", "post"))
	m.AssertExpectations(t)
}

func TestValidator_HasRequiredBodyParam_TrueWhenRequired(t *testing.T) {
	m := new(MockSpecProvider)
	v := NewValidator(m)

	op := &openapi3.Operation{
		RequestBody: &openapi3.RequestBodyRef{
			Value: &openapi3.RequestBody{Required: true},
		},
	}

	m.On("FindOperation", "/x", "post").Return(op).Once()

	require.True(t, v.HasRequiredBodyParam("/x", "post"))
	m.AssertExpectations(t)
}

func TestValidator_HasRequiredBodyParam_FalseWhenNoRequestBody(t *testing.T) {
	m := new(MockSpecProvider)
	v := NewValidator(m)

	op := &openapi3.Operation{Responses: openapi3.NewResponses()}
	m.On("FindOperation", "/x", "post").Return(op).Once()

	require.False(t, v.HasRequiredBodyParam("/x", "post"))
	m.AssertExpectations(t)
}

func TestValidator_HasRequiredBodyParam_FalseWhenRequestBodyValueNil(t *testing.T) {
	m := new(MockSpecProvider)
	v := NewValidator(m)

	op := &openapi3.Operation{
		RequestBody: &openapi3.RequestBodyRef{}, // Value nil
		Responses:   openapi3.NewResponses(),
	}
	m.On("FindOperation", "/x", "post").Return(op).Once()

	require.False(t, v.HasRequiredBodyParam("/x", "post"))
	m.AssertExpectations(t)
}

func TestValidator_HasRequiredBodyParam_FalseWhenNotRequired(t *testing.T) {
	m := new(MockSpecProvider)
	v := NewValidator(m)

	op := &openapi3.Operation{
		RequestBody: &openapi3.RequestBodyRef{
			Value: &openapi3.RequestBody{Required: false},
		},
		Responses: openapi3.NewResponses(),
	}
	m.On("FindOperation", "/x", "post").Return(op).Once()

	require.False(t, v.HasRequiredBodyParam("/x", "post"))
	m.AssertExpectations(t)
}

func TestValidator_IsEmptyBody_NilBodyIsEmpty(t *testing.T) {
	m := new(MockSpecProvider)
	v := NewValidator(m)

	req, _ := http.NewRequest("POST", "http://example.com", nil)
	req.Body = nil

	empty, err := v.IsEmptyBody(req)
	require.NoError(t, err)
	require.True(t, empty)
}

func TestValidator_IsEmptyBody_EmptyBytesIsEmptyAndRewindable(t *testing.T) {
	m := new(MockSpecProvider)
	v := NewValidator(m)

	req, _ := http.NewRequest("POST", "http://example.com",
		io.NopCloser(bytes.NewReader([]byte{})))

	empty, err := v.IsEmptyBody(req)
	require.NoError(t, err)
	require.True(t, empty)

	b, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, "", string(b))
}

func TestValidator_IsEmptyBody_WhitespaceOnlyIsEmptyAndRewindable(t *testing.T) {
	m := new(MockSpecProvider)
	v := NewValidator(m)

	body := " \n\t  "
	req, _ := http.NewRequest("POST", "http://example.com", io.NopCloser(strings.NewReader(body)))

	empty, err := v.IsEmptyBody(req)
	require.NoError(t, err)
	require.True(t, empty)

	b, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, body, string(b))
}

func TestValidator_IsEmptyBody_NonEmptyIsNotEmptyAndRewindable(t *testing.T) {
	m := new(MockSpecProvider)
	v := NewValidator(m)

	body := `{"a":1}`
	req, _ := http.NewRequest("POST", "http://example.com", io.NopCloser(strings.NewReader(body)))

	empty, err := v.IsEmptyBody(req)
	require.NoError(t, err)
	require.False(t, empty)

	b, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, body, string(b))
}

func TestValidator_IsEmptyBody_ReadError(t *testing.T) {
	m := new(MockSpecProvider)
	v := NewValidator(m)

	req, _ := http.NewRequest("POST", "http://example.com", io.NopCloser(errReader{}))

	_, err := v.IsEmptyBody(req)
	require.Error(t, err)
}
