package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

type HTTPServer struct {
	server *http.Server
	logger *slog.Logger
}

func NewHTTPServer(addr string, logger *slog.Logger) *HTTPServer {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	return &HTTPServer{
		server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
		logger: logger,
	}
}

func (h *HTTPServer) Start() error {
	h.logger.Info("HTTP сервер запущен", "addr", h.server.Addr)
	return h.server.ListenAndServe()
}

func (h *HTTPServer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return h.server.Shutdown(ctx)
}
