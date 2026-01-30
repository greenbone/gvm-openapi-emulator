package samples

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ozgen/openapi-sample-emulator/logger"

	"github.com/ozgen/openapi-sample-emulator/config"
	"github.com/ozgen/openapi-sample-emulator/utils"
)

type ResolverConfig struct {
	BaseDir string
	Layout  config.LayoutMode
	State   string
}

func ResolveSamplePath(
	cfg ResolverConfig,
	method, swaggerTpl, actualPath, legacyFlatFilename string,
	scenarioEnabled bool,
	scenarioFilename string,
	engine ScenarioResolver,
) (string, error) {
	method = strings.ToUpper(method)
	log := logger.GetLogger()

	// Scenario priority (hardcoded)
	if scenarioEnabled {
		scPath := ScenarioPathForSwagger(cfg.BaseDir, swaggerTpl, scenarioFilename)
		if utils.FileExists(scPath) {
			sc, err := LoadScenario(scPath)
			if err != nil {
				log.WithError(err).Error("failed to load scenario")
				return "", fmt.Errorf("load scenario %s: %w", scPath, err)
			}
			if engine == nil {
				log.WithError(err).Error("failed to load scenario")
				return "", fmt.Errorf("scenario enabled but engine is nil")
			}
			file, _, err := engine.ResolveScenarioFile(sc, method, swaggerTpl, actualPath)
			if err != nil {
				log.WithError(err).Error("failed to resolve scenario")
				return "", fmt.Errorf("scenario resolve: %w", err)
			}
			full := filepath.Join(filepath.Dir(scPath), file)
			if utils.FileExists(full) {
				return full, nil
			}
			log.WithError(err).Error("failed to resolve scenario")
			return "", fmt.Errorf("scenario file not found: %s", full)
		}
	}

	// Non-scenario fallback: folder/flat like before
	relCandidates := buildCandidates(cfg, method, swaggerTpl, legacyFlatFilename)
	if len(relCandidates) == 0 {
		log.Warn("no candidates found")
		return "", fmt.Errorf("no candidates for method=%s path=%s", method, swaggerTpl)
	}

	for _, rel := range relCandidates {
		full := filepath.Join(cfg.BaseDir, rel)
		if utils.FileExists(full) {
			return full, nil
		}
	}

	log.WithField("path", actualPath).Info("no candidates found, spec example will return")
	return "", fmt.Errorf("no sample file found (tried: %v)", relCandidates)
}

func buildCandidates(cfg ResolverConfig, method, swaggerPath, legacyFlatFilename string) []string {
	layout := cfg.Layout
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
