// SPDX-FileCopyrightText: 2026 Greenbone AG
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"github.com/ozgen/gvm-openapi-emulator/config"
	"github.com/ozgen/gvm-openapi-emulator/internal/server"
	"github.com/ozgen/gvm-openapi-emulator/logger"
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
