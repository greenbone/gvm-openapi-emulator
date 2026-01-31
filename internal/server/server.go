package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ozgen/openapi-sample-emulator/config"
	"github.com/ozgen/openapi-sample-emulator/internal/openapi"
	"github.com/ozgen/openapi-sample-emulator/internal/samples"
	"github.com/ozgen/openapi-sample-emulator/logger"
	"github.com/ozgen/openapi-sample-emulator/utils"
	"github.com/sirupsen/logrus"
)

type Config struct {
	Port           string
	SpecPath       string
	SamplesDir     string
	FallbackMode   config.FallbackMode
	ValidationMode config.ValidationMode
	Layout         config.LayoutMode
}

type Server struct {
	cfg            Config
	specProvider   openapi.ISpecProvider
	routerProvider openapi.IRouterProvider
	validator      openapi.IValidator
	log            *logrus.Logger

	scenario samples.ScenarioResolver
}

func New(cfg Config) (*Server, error) {
	log := logger.GetLogger()
	specProvider, err := openapi.NewSpecProvider(cfg.SpecPath, log)
	if err != nil {
		return nil, err
	}

	routeProvider := openapi.NewRouterProvider(specProvider.GetSpec())
	validator := openapi.NewValidator(specProvider)

	if strings.TrimSpace(string(cfg.Layout)) == "" {
		cfg.Layout = config.LayoutAuto
	}

	s := &Server{
		cfg:            cfg,
		specProvider:   specProvider,
		routerProvider: routeProvider,
		validator:      validator,
		log:            log,
	}

	if config.Envs.Scenario.Enabled {
		s.scenario = samples.NewScenarioEngine()
	}

	return s, nil
}

func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handle)

	addr := "0.0.0.0:" + s.cfg.Port

	s.log.Printf("mock listening on %s", addr)
	s.log.Printf(
		"spec=%s samples=%s fallback=%s validation=%s layout=%s scenario_enabled=%v scenario_file=%q",
		s.cfg.SpecPath, s.cfg.SamplesDir, s.cfg.FallbackMode, s.cfg.ValidationMode,
		s.cfg.Layout, config.Envs.Scenario.Enabled, config.Envs.Scenario.Filename,
	)

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return server.ListenAndServe()
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	method := r.Method
	path := r.URL.Path

	// Health endpoints
	if method == http.MethodGet && (path == "/health/alive" || path == "/health/ready" || path == "/health/started") {
		utils.WriteJSON(w, 200, map[string]any{"ok": true})
		return
	}

	rt := s.routerProvider.FindRoute(method, path)
	if rt == nil {
		utils.WriteJSON(w, 404, map[string]any{
			"error":  "No route",
			"method": method,
			"path":   path,
		})
		return
	}

	if s.cfg.ValidationMode == config.ValidationRequired {
		if s.validator.HasRequiredBodyParam(rt.Swagger, rt.Method) {
			empty, err := s.validator.IsEmptyBody(r)
			if err != nil {
				utils.WriteJSON(w, 400, map[string]any{"error": "Bad Request", "details": err.Error()})
				return
			}
			if empty {
				utils.WriteJSON(w, 400, map[string]any{
					"error":   "Bad Request",
					"details": "Request body is required by the API spec",
				})
				return
			}
		}
	}

	resp, err := samples.LoadResolved(
		s.cfg.SamplesDir,
		method,
		rt.Swagger,
		path,
		rt.SampleFile,
		s.cfg.Layout,
		config.Envs.Scenario.Enabled,
		config.Envs.Scenario.Filename,
		s.scenario,
	)
	if err != nil {
		if s.cfg.FallbackMode == config.FallbackOpenAPIExample {
			if body, ok := s.specProvider.TryGetExampleBody(rt.Swagger, rt.Method); ok {
				w.Header().Set("content-type", "application/json")
				w.WriteHeader(200)
				_, _ = w.Write(body)
				return
			}
		}

		utils.WriteJSON(w, 501, map[string]any{
			"error":              "No sample file for route",
			"method":             method,
			"path":               path,
			"swaggerPath":        rt.Swagger,
			"legacyFlatFilename": rt.SampleFile,
			"layout":             s.cfg.Layout,
			"details":            err.Error(),
			"hint":               "Create the sample file under SAMPLES_DIR/<path>/<METHOD>[.<state>].json (or legacy flat), or set FALLBACK_MODE=openapi_examples and add examples to swagger.json",
		})
		return
	}

	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}
	w.WriteHeader(resp.Status)
	_, _ = w.Write(resp.Body)
}

func (s *Server) DebugRoutes() string {
	out := ""
	for _, r := range s.routerProvider.GetRoutes() {
		out += fmt.Sprintf("%s %s -> %s\n", r.Method, r.Swagger, r.SampleFile)
	}
	return out
}
