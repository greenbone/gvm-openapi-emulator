// SPDX-FileCopyrightText: 2026 Greenbone AG
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"github.com/joho/godotenv"
	"github.com/ozgen/gvm-openapi-emulator/utils"
)

type RunningEnv string

const (
	EnvK8s    RunningEnv = "k8s"
	EnvDocker RunningEnv = "docker"
	EnvLocal  RunningEnv = "local"
)

type FallbackMode string

const (
	FallbackNone           FallbackMode = "none"
	FallbackOpenAPIExample FallbackMode = "openapi_examples"
)

type ValidationMode string

const (
	ValidationNone     ValidationMode = "none"
	ValidationRequired ValidationMode = "required"
)

type LayoutMode string

const (
	LayoutAuto    LayoutMode = "auto"    // folder-first, then flat
	LayoutFolders LayoutMode = "folders" // only folders
	LayoutFlat    LayoutMode = "flat"    // only flat
)

type ScenarioConfig struct {
	Enabled  bool
	Filename string
}

type Config struct {
	ServerPort     string
	SpecPath       string
	SamplesDir     string
	LogLevel       string
	RunningEnv     RunningEnv
	FallbackMode   FallbackMode
	DebugRoutes    bool
	ValidationMode ValidationMode
	Layout         LayoutMode

	Scenario ScenarioConfig
}

var Envs = initConfig()

func initConfig() Config {
	_ = godotenv.Load()

	return Config{
		ServerPort:     utils.GetEnv("SERVER_PORT", "8086"),
		SpecPath:       utils.GetEnv("SPEC_PATH", "/work/swagger.json"),
		SamplesDir:     utils.GetEnv("SAMPLES_DIR", "/work/sample"),
		LogLevel:       utils.GetEnv("LOG_LEVEL", "info"),
		RunningEnv:     RunningEnv(utils.GetEnv("RUNNING_ENV", "docker")),
		ValidationMode: ValidationMode(utils.GetEnv("VALIDATION_MODE", "required")),
		FallbackMode:   FallbackMode(utils.GetEnv("FALLBACK_MODE", "openapi_examples")),
		DebugRoutes:    utils.GetEnvAsBool("DEBUG_ROUTES", false),
		Layout:         LayoutMode(utils.GetEnv("LAYOUT_MODE", "auto")),

		Scenario: ScenarioConfig{
			Enabled:  utils.GetEnvAsBool("SCENARIO_ENABLED", true),
			Filename: utils.GetEnv("SCENARIO_FILENAME", "scenario.json"),
		},
	}
}
