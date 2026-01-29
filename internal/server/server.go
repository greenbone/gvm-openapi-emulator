package server

import (
	"bytes"
	"fmt"
	"io"
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
	cfg    Config
	spec   *openapi.Spec
	routes []openapi.Route
	log    *logrus.Logger

	flow       *StateFlow
	bodyStates []string
}

func New(cfg Config) (*Server, error) {
	spec, err := openapi.LoadSpec(cfg.SpecPath)
	if err != nil {
		return nil, err
	}
	routes := openapi.BuildRoutes(spec)

	if strings.TrimSpace(string(cfg.Layout)) == "" {
		cfg.Layout = config.LayoutAuto
	}

	s := &Server{
		cfg:    cfg,
		spec:   spec,
		routes: routes,
		log:    logger.GetLogger(),

		flow: NewStateFlow(StateFlowConfig{
			FlowSpec:        config.Envs.StateFlow,        // e.g. "requested,running*9,succeeded"
			StepSeconds:     config.Envs.StateStepSeconds, // time-based
			StepCalls:       config.Envs.StateStepCalls,   // count-based (wins if >0)
			DefaultStepSecs: 2,
			ResetOnLast:     config.Envs.StateResetOnLast,
		}),
		bodyStates: ParseBodyStateRules(config.Envs.BodyStates), // e.g. "start,stop"
	}

	return s, nil
}

func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handle)

	addr := "0.0.0.0:" + s.cfg.Port

	s.log.Printf("mock listening on %s", addr)
	s.log.Printf(
		"spec=%s samples=%s fallback=%s validation=%s layout=%s state_flow=%q step_seconds=%d step_calls=%d id_param=%q body_states=%q",
		s.cfg.SpecPath,
		s.cfg.SamplesDir,
		s.cfg.FallbackMode,
		s.cfg.ValidationMode,
		s.cfg.Layout,
		config.Envs.StateFlow,
		config.Envs.StateStepSeconds,
		config.Envs.StateStepCalls,
		config.Envs.StateIDParam,
		config.Envs.BodyStates,
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

	rt := openapi.FindRoute(s.routes, method, path)
	if rt == nil {
		utils.WriteJSON(w, 404, map[string]any{
			"error":  "No route",
			"method": method,
			"path":   path,
		})
		return
	}

	if s.cfg.ValidationMode == config.ValidationRequired {
		if openapi.HasRequiredBodyParam(s.spec, rt.Swagger, rt.Method) {
			empty, err := openapi.IsEmptyBody(r)
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

	// Default state from flow
	state := ""
	if s.flow != nil && s.flow.Enabled() {
		key := makeStateKey(method, rt.Swagger, path, config.Envs.StateIDParam)
		state = s.flow.Current(key)
	}

	// Override with body-based state selection
	// If body contains "start" or "stop", we set state to that token.
	if len(s.bodyStates) > 0 {
		body, err := ReadBodyAndRestore(r)
		if err == nil {
			if st, ok := StateFromBodyContains(body, s.bodyStates); ok {
				state = st
			}
		}
	}

	resp, err := samples.LoadResolved(
		s.cfg.SamplesDir,
		method,
		rt.Swagger,
		rt.SampleFile,
		state,
		s.cfg.Layout,
	)
	if err != nil {
		if s.cfg.FallbackMode == config.FallbackOpenAPIExample {
			if body, ok := openapi.TryGetExampleBody(s.spec, rt.Swagger, rt.Method); ok {
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
			"state":              state,
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
	for _, r := range s.routes {
		out += fmt.Sprintf("%s %s -> %s\n", r.Method, r.Swagger, r.SampleFile)
	}
	return out
}

func ReadBodyAndRestore(r *http.Request) (string, error) {
	if r.Body == nil {
		return "", nil
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	r.Body = io.NopCloser(bytes.NewReader(b))
	return string(b), nil
}
