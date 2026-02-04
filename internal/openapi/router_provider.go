// SPDX-FileCopyrightText: 2026 Greenbone AG
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package openapi

import (
	"fmt"
	"regexp"
	"strings"
)

type RouterProvider struct {
	routes []Route
}

func NewRouterProvider(spec *Spec) IRouterProvider {
	if spec == nil || spec.Doc3 == nil || spec.Doc3.Paths == nil {
		return nil
	}

	var out []Route
	for swaggerPath, item := range spec.Doc3.Paths.Map() {
		if item == nil {
			continue
		}

		for method := range item.Operations() {
			m := strings.ToUpper(method)
			out = append(out, Route{
				Method:     m,
				Swagger:    swaggerPath,
				Regex:      swaggerPathToRegex(swaggerPath),
				SampleFile: swaggerPathToSampleName(m, swaggerPath),
			})
		}
	}
	return &RouterProvider{routes: out}
}

func (p *RouterProvider) FindRoute(method, path string) *Route {
	method = strings.ToUpper(method)

	var best *Route
	bestScore := -1

	for i := range p.routes {
		r := &p.routes[i]
		if r.Method != method {
			continue
		}
		if !r.Regex.MatchString(path) {
			continue
		}

		score := p.routeSpecificityScore(r.Swagger)
		if score > bestScore {
			best = r
			bestScore = score
		}
	}

	return best
}

func (p *RouterProvider) GetRoutes() []Route {
	return p.routes
}

func (p *RouterProvider) routeSpecificityScore(swaggerPath string) int {
	parts := strings.Split(strings.Trim(swaggerPath, "/"), "/")
	score := 0
	for _, p := range parts {
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
			score += 0 // param segment
		} else {
			score += 10
		}
	}

	score += len(parts)
	return score
}

func swaggerPathToSampleName(method, swaggerPath string) string {
	s := strings.TrimPrefix(swaggerPath, "/")
	s = strings.ReplaceAll(s, "/", "_")
	return fmt.Sprintf("%s__%s.json", strings.ToUpper(method), s)
}

func swaggerPathToRegex(swaggerPath string) *regexp.Regexp {
	parts := strings.Split(swaggerPath, "/")
	var out []string

	for _, p := range parts {
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
			out = append(out, `([^/]+)`)
		} else {
			out = append(out, regexp.QuoteMeta(p))
		}
	}

	pat := "^/" + strings.Join(out, "/") + "/?$"
	return regexp.MustCompile(pat)
}
