// Package httpapi implements Sandrone's HTTP API entrypoint.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/buildinfo"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type Option func(*Server)

type Server struct {
	rt         *app.Runtime
	mux        *http.ServeMux
	mcpPath    string
	mcpHandler http.Handler
	webUI      http.Handler
}

func WithMCP(path string, handler http.Handler) Option {
	return func(s *Server) {
		s.mcpPath = path
		s.mcpHandler = handler
	}
}

func WithWebUI(handler http.Handler) Option {
	return func(s *Server) {
		s.webUI = handler
	}
}

func New(rt *app.Runtime, opts ...Option) *Server {
	s := &Server{rt: rt, mux: http.NewServeMux()}
	for _, opt := range opts {
		opt(s)
	}
	s.routes()
	return s
}

func (s *Server) Name() string { return "http" }

func (s *Server) Handler() http.Handler {
	managed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := validateDeleteResourcePath(r); err != nil {
			writeError(w, err, http.StatusBadRequest)
			return
		}
		s.mux.ServeHTTP(w, r)
	})
	authenticated := s.auth(managed)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.publicPath(r.URL.Path) {
			s.mux.ServeHTTP(w, r)
			return
		}
		authenticated.ServeHTTP(w, r)
	})
	return s.logRequests(handler)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		level := slog.LevelDebug
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}
		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"bytes", rec.bytes,
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		}
		if requestID := r.Header.Get("X-Request-Id"); requestID != "" {
			attrs = append(attrs, "request_id", requestID)
		}
		s.rt.Logger.Log(r.Context(), level, "http request completed", attrs...)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.status != 0 {
		return
	}
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(body)
	r.bytes += n
	return n, err
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func (s *Server) Run(ctx context.Context, rt *app.Runtime) error {
	if rt != nil {
		s.rt = rt
	}
	srv := &http.Server{
		Addr:              s.rt.Config.HTTP.Listen,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		s.rt.Logger.Info("starting HTTP API", "listen", srv.Addr)
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.rt.Logger.Error("HTTP API stopped with error", "error", err)
		}
		errCh <- err
	}()
	select {
	case <-ctx.Done():
		s.rt.Logger.Info("stopping HTTP API")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			s.rt.Logger.Error("HTTP API shutdown failed", "error", err)
			return err
		}
		s.rt.Logger.Info("HTTP API stopped")
		return ctx.Err()
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("GET /version", s.version)
	s.mux.HandleFunc("GET /convert", s.publicConvert)
	s.mux.HandleFunc("/v1/convert", methodNotAllowed)
	s.mux.HandleFunc("POST /v1/validate", s.validate)
	s.mux.HandleFunc("GET /v1/inspect", s.inspect)
	s.mux.HandleFunc("GET /v1/backup", s.getBackup)
	s.mux.HandleFunc("POST /v1/backup/restore", s.restoreBackup)
	s.mux.HandleFunc("GET /v1/subscriptions", s.listSubscriptions)
	s.mux.HandleFunc("POST /v1/subscriptions", s.putSubscription)
	s.mux.HandleFunc("GET /v1/subscriptions/", s.getSubscription)
	s.mux.HandleFunc("POST /v1/subscriptions/", s.subscriptionAction)
	s.mux.HandleFunc("DELETE /v1/subscriptions/", s.deleteSubscription)
	s.mux.HandleFunc("GET /v1/files", s.listFiles)
	s.mux.HandleFunc("POST /v1/files", s.putFile)
	s.mux.HandleFunc("GET /v1/files/", s.getFile)
	s.mux.HandleFunc("DELETE /v1/files/", s.deleteFile)
	s.mux.HandleFunc("GET /v1/settings/runtime", s.getRuntimeSettings)
	s.mux.HandleFunc("PUT /v1/settings/runtime", s.putRuntimeSettings)
	s.mux.HandleFunc("GET /v1/rule-set-catalog", s.listRuleSetCatalog)
	s.mux.HandleFunc("GET /v1/shares", s.listShares)
	s.mux.HandleFunc("POST /v1/shares", s.createShare)
	s.mux.HandleFunc("GET /v1/shares/", s.getShare)
	s.mux.HandleFunc("DELETE /v1/shares/", s.deleteShare)
	s.mux.HandleFunc("GET /s/", s.publicShare)
	if s.mcpHandler != nil {
		path := s.mcpPath
		if path == "" {
			path = app.DefaultMCPPath
		}
		s.mux.Handle(path, s.mcpHandler)
		s.mux.Handle(path+"/", s.mcpHandler)
	}
	if s.webUI != nil {
		s.mux.Handle("/", s.webUI)
	}
}

func methodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

func (s *Server) auth(next http.Handler) http.Handler {
	if !app.TokenRequired(s.rt.Config.HTTP) {
		return next
	}
	token := s.rt.Config.HTTP.Token
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" || r.Header.Get("Authorization") != "Bearer "+token {
			writeError(w, domain.NewError(domain.CodeInvalidArgument, "permission denied"), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) publicPath(path string) bool {
	if path == "/healthz" || path == "/version" || path == "/convert" || strings.HasPrefix(path, "/s/") {
		return true
	}
	if s.webUI == nil {
		return false
	}
	return !s.protectedPath(path)
}

func (s *Server) protectedPath(path string) bool {
	if path == "/v1" || strings.HasPrefix(path, "/v1/") {
		return true
	}
	mcpPath := s.mcpPath
	if mcpPath == "" {
		mcpPath = app.DefaultMCPPath
	}
	return path == mcpPath || strings.HasPrefix(path, mcpPath+"/")
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"name": "sandrone", "version": buildinfo.Version()})
}
