package samples

import (
	"encoding/json"
	"fmt"
	"github.com/ozgen/openapi-sample-emulator/config"
	"os"
	"path/filepath"
	"strings"
)

type Envelope struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    any               `json:"body"`
}

// Response is what the server will write to the client.
type Response struct {
	Status  int
	Headers map[string]string
	Body    []byte
}

// LoadResolved loads a sample response using folder-first resolution, then legacy flat filename.
func LoadResolved(baseDir, method, swaggerPath, legacyFlatFilename, state string, mode config.LayoutMode) (*Response, error) {
	p, err := ResolveSamplePath(ResolverConfig{
		BaseDir: baseDir,
		Layout:  mode,
		State:   state,
	}, method, swaggerPath, legacyFlatFilename)
	if err != nil {
		return nil, err
	}
	return loadFile(p)
}

// Load keeps the old behavior.
func Load(dir, fileName string) (*Response, error) {
	p := filepath.Join(dir, fileName)
	return loadFile(p)
}

func loadFile(path string) (*Response, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read sample %s: %w", path, err)
	}
	raw := strings.TrimSpace(string(b))
	if raw == "" {
		return &Response{
			Status:  200,
			Headers: map[string]string{"content-type": "application/json"},
			Body:    []byte("{}"),
		}, nil
	}

	if isJSONObject(raw) && looksLikeEnvelope([]byte(raw)) {
		var env Envelope
		if err := json.Unmarshal([]byte(raw), &env); err == nil {
			status := env.Status
			if status == 0 {
				status = 200
			}

			headers := env.Headers
			if headers == nil {
				headers = map[string]string{}
			}

			// Default content-type if not provided
			if _, ok := headerGet(headers, "content-type"); !ok {
				headers["content-type"] = "application/json"
			}

			var bodyBytes []byte
			if env.Body == nil {
				bodyBytes = []byte("{}")
			} else {
				bodyBytes, err = json.Marshal(env.Body)
				if err != nil {
					return nil, fmt.Errorf("marshal envelope body: %w", err)
				}
			}

			return &Response{
				Status:  status,
				Headers: headers,
				Body:    bodyBytes,
			}, nil
		}
	}

	return &Response{
		Status:  200,
		Headers: map[string]string{"content-type": "application/json"},
		Body:    []byte(raw),
	}, nil
}

func isJSONObject(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")
}

func looksLikeEnvelope(raw []byte) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil || len(m) == 0 {
		return false
	}
	_, hasStatus := m["status"]
	_, hasHeaders := m["headers"]
	_, hasBody := m["body"]
	return hasStatus || hasHeaders || hasBody
}

func headerGet(h map[string]string, key string) (string, bool) {
	lk := strings.ToLower(key)
	for k, v := range h {
		if strings.ToLower(k) == lk {
			return v, true
		}
	}
	return "", false
}
