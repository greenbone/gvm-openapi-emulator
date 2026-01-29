package config

import (
	"github.com/joho/godotenv"
	"github.com/ozgen/openapi-sample-emulator/utils"
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

type Config struct {
	ServerPort       string
	SpecPath         string
	SamplesDir       string
	LogLevel         string
	RunningEnv       RunningEnv
	FallbackMode     FallbackMode
	DebugRoutes      bool
	ValidationMode   ValidationMode
	Layout           LayoutMode
	StateFlow        string // e.g. "requested,running*9,succeeded"
	StateStepSeconds int    // time-based progression
	StateStepCalls   int    // count-based progression (if >0, overrides seconds)
	StateIDParam     string // e.g. "scan_id"
	StateResetOnLast bool
	BodyStates       string
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

		StateFlow:        utils.GetEnv("STATE_FLOW", ""),
		StateStepSeconds: utils.GetEnvAsInt("STATE_STEP_SECONDS", 0),
		StateStepCalls:   utils.GetEnvAsInt("STATE_STEP_CALLS", 1),
		StateResetOnLast: utils.GetEnvAsBool("STATE_RESET_ON_LAST", false),
		StateIDParam:     utils.GetEnv("STATE_ID_PARAM", "id"),
		BodyStates:       utils.GetEnv("BODY_STATES", "start,stop"),
	}
}
