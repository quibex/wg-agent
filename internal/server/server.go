package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"

	"github.com/quibex/wg-agent/internal/config"
	"github.com/quibex/wg-agent/internal/ratelimit"
	"github.com/quibex/wg-agent/internal/wireguard"
	proto "github.com/quibex/wg-agent/pkg/api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Server основной сервер wg-agent
type Server struct {
	config     *config.Config
	logger     *slog.Logger
	wgClient   wireguard.Client
	limiter    *ratelimit.Limiter
	httpServer *HTTPServer
}

// New создает новый сервер
func New(cfg *config.Config, log *slog.Logger, wgClient wireguard.Client) *Server {
	return &Server{
		config:     cfg,
		logger:     log,
		wgClient:   wgClient,
		limiter:    ratelimit.NewLimiter(cfg.RateLimit),
		httpServer: NewHTTPServer(cfg.HTTPAddr, log),
	}
}

// Start запускает сервер
func (s *Server) Start() error {
	s.logger.Info("Starting wg-agent", "addr", s.config.Addr)

	s.logger.Info("Starting HTTP server", "addr", s.config.HTTPAddr)

	go func() {
		if err := s.httpServer.Start(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error", "error", err)
		}
	}()

	// Настройка TLS
	tlsConfig, err := s.setupTLS()
	if err != nil {
		return fmt.Errorf("TLS setup failed: %w", err)
	}

	listener, err := net.Listen("tcp", s.config.Addr)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	// Создание gRPC сервера с TLS и rate-limit interceptor
	creds := credentials.NewTLS(tlsConfig)
	grpcServer := grpc.NewServer(
		grpc.Creds(creds),
		grpc.UnaryInterceptor(s.limiter.UnaryInterceptor()),
	)

	// Регистрация сервиса
	proto.RegisterWireGuardAgentServer(grpcServer, newAgentService(
		s.logger,
		s.wgClient,
		s.config.Interface,
		s.config.Subnet,
		s.config.ServerEndpoint(),
	))

	s.logger.Info("gRPC server started", "addr", s.config.Addr)

	return grpcServer.Serve(listener)
}

// setupTLS настраивает TLS конфигурацию с client certificate authentication
func (s *Server) setupTLS() (*tls.Config, error) {
	// Загрузка серверного сертификата
	cert, err := tls.LoadX509KeyPair(s.config.TLSCert, s.config.TLSKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}

	// Загрузка CA для проверки клиентских сертификатов
	caCert, err := os.ReadFile(s.config.CABundle)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA bundle: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to add CA certificate")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCertPool,
		MinVersion:   tls.VersionTLS13, // TLS 1.3 согласно ТЗ
	}, nil
}

// Stop останавливает сервер
func (s *Server) Stop() error {
	s.logger.Info("Stopping server")

	if s.httpServer != nil {
		if err := s.httpServer.Stop(); err != nil {
			s.logger.Error("HTTP server stop error", "error", err)
		}
	}

	if s.wgClient != nil {
		return s.wgClient.Close()
	}
	return nil
}
