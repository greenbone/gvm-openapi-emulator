package samples

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ozgen/openapi-sample-emulator/utils"
	"github.com/sirupsen/logrus"

	"github.com/ozgen/openapi-sample-emulator/config"
)

type SampleProvider struct {
	cfg ProviderConfig
	log *logrus.Logger
}

func NewSampleProvider(cfg ProviderConfig, log *logrus.Logger) ISampleProvider {
	return &SampleProvider{cfg: cfg, log: log}
}

func (p *SampleProvider) ResolveAndLoad(method, swaggerTpl, actualPath, legacyFlatFilename string) (*Response, error) {
	path, err := p.ResolvePath(method, swaggerTpl, actualPath, legacyFlatFilename)
	if err != nil {
		p.log.WithError(err).Info("failed to resolve path")
		return nil, err
	}
	return loadFile(path)
}

func (p *SampleProvider) ResolvePath(method, swaggerTpl, actualPath, legacyFlatFilename string) (string, error) {
	cfg := p.cfg
	method = strings.ToUpper(method)

	// Scenario priority
	if cfg.ScenarioEnabled {
		scPath := ScenarioPathForSwagger(cfg.BaseDir, swaggerTpl, cfg.ScenarioFilename)
		if utils.FileExists(scPath) {
			sc, err := LoadScenario(scPath)
			if err != nil {
				p.log.WithError(err).Warn("failed to load scenario")
				return "", fmt.Errorf("load scenario %s: %w", scPath, err)
			}
			if cfg.Engine == nil {
				return "", fmt.Errorf("scenario enabled but engine is nil")
			}

			file, _, err := cfg.Engine.ResolveScenarioFile(sc, method, swaggerTpl, actualPath)
			if err != nil {
				p.log.WithError(err).Warn("failed to resolve scenario")
				return "", fmt.Errorf("scenario resolve: %w", err)
			}

			full := filepath.Join(filepath.Dir(scPath), file)
			if utils.FileExists(full) {
				return full, nil
			}
			return "", fmt.Errorf("scenario file not found: %s", full)
		}
		if cfg.ScenarioEnabled && cfg.Engine != nil {
			_ = cfg.Engine.TryResetByRequest(method, actualPath)
		}
	}

	// Non-scenario fallback: folder/flat
	candidates := buildCandidates(cfg.Layout, method, swaggerTpl, legacyFlatFilename)
	if len(candidates) == 0 {
		return "", fmt.Errorf("no candidates for method=%s path=%s", method, swaggerTpl)
	}

	for _, rel := range candidates {
		full := filepath.Join(cfg.BaseDir, rel)
		if utils.FileExists(full) {
			return full, nil
		}
	}

	p.log.WithField("path", actualPath).Info("no sample found; caller may fallback to spec example")
	return "", fmt.Errorf("no sample file found (tried: %v)", candidates)
}

func buildCandidates(layout config.LayoutMode, method, swaggerPath, legacyFlatFilename string) []string {
	if layout == "" {
		layout = config.LayoutAuto
	}

	var out []string
	if layout == config.LayoutAuto || layout == config.LayoutFolders {
		pathDir := strings.TrimPrefix(swaggerPath, "/")
		pathDir = filepath.FromSlash(pathDir)
		out = append(out, filepath.Join(pathDir, fmt.Sprintf("%s.json", method)))
	}
	if layout == config.LayoutAuto || layout == config.LayoutFlat {
		out = append(out, legacyFlatFilename)
	}
	return out
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
