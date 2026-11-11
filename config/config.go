// SPDX-FileCopyrightText: 2026 Greenbone AG
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"time"

	"github.com/greenbone/gvm-openapi-emulator/utils"
	"github.com/joho/godotenv"
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

type Profile string

const (
	ProfileGeneric  Profile = "generic"
	ProfileOpenVASD Profile = "openvasd"
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
	Profile        Profile
	ResetToken     string
	TimeoutDelay   time.Duration

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
		Profile:        Profile(utils.GetEnv("PROFILE", "generic")),
		ResetToken:     utils.GetEnv("OPENVASD_RESET_TOKEN", ""),
		TimeoutDelay:   getEnvDuration("OPENVASD_TIMEOUT_DELAY", 11*time.Second),

		Scenario: ScenarioConfig{
			Enabled:  utils.GetEnvAsBool("SCENARIO_ENABLED", true),
			Filename: utils.GetEnv("SCENARIO_FILENAME", "scenario.json"),
		},
	}
}

func getEnvDuration(name string, fallback time.Duration) time.Duration {
	value := utils.GetEnv(name, fallback.String())
	duration, err := time.ParseDuration(value)
	if err != nil || duration < 0 {
		return fallback
	}
	return duration
}
