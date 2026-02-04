// SPDX-FileCopyrightText: 2026 Greenbone AG
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package openapi

import (
	"bytes"
	"io"
	"net/http"
	"strings"
)

type Validator struct {
	spec ISpecProvider
}

func NewValidator(provider ISpecProvider) IValidator {
	return &Validator{
		spec: provider,
	}
}

func (v *Validator) HasRequiredBodyParam(swaggerPath, method string) bool {
	op := v.spec.FindOperation(swaggerPath, method)
	if op == nil || op.RequestBody == nil || op.RequestBody.Value == nil {
		return false
	}
	return op.RequestBody.Value.Required
}

func (v *Validator) IsEmptyBody(r *http.Request) (bool, error) {
	if r.Body == nil {
		return true, nil
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return false, err
	}
	r.Body = io.NopCloser(bytes.NewReader(b))
	return len(strings.TrimSpace(string(b))) == 0, nil
}
