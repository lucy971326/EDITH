// Package httpserver 提供 EDITH 的 HTTP 路由挂载。
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github-agent/edith/internal/identity"
)

type Server struct {
	httpSrv *http.Server
	logger  *log.Logger
}

func New(addr string, logger *log.Logger) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler)
	mux.Handle("/api/me", identity.Middleware()(http.HandlerFunc(meHandler)))

	return &Server{
		httpSrv: &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
		},
		logger: logger,
	}
}

func (s *Server) ListenAndServe() error {
	s.logger.Printf("httpserver: listening on %s", s.httpSrv.Addr)
	if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("httpserver: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Printf("httpserver: shutting down")
	return s.httpSrv.Shutdown(ctx)
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"})
}