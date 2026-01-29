package main

import (
	"github.com/ozgen/openapi-sample-emulator/config"
	"github.com/ozgen/openapi-sample-emulator/internal/server"
	"github.com/ozgen/openapi-sample-emulator/logger"
)

func main() {
	cfg := config.Envs
	log := logger.GetLogger()

	srv, err := server.New(server.Config{
		Port:           cfg.ServerPort,
		SpecPath:       cfg.SpecPath,
		SamplesDir:     cfg.SamplesDir,
		FallbackMode:   cfg.FallbackMode,
		ValidationMode: cfg.ValidationMode,
		Layout:         cfg.Layout,
	})
	if err != nil {
		log.Fatalf("failed to init server: %v", err)
	}

	if cfg.DebugRoutes {
		log.Print("\n" + srv.DebugRoutes())
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
