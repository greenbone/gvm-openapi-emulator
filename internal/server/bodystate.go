package server

import "strings"

func ParseBodyStateRules(spec string) []string {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil
	}

	parts := strings.Split(spec, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func StateFromBodyContains(body string, states []string) (string, bool) {
	for _, st := range states {
		if strings.Contains(body, st) {
			return st, true
		}
	}
	return "", false
}
