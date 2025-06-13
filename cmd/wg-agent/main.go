package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/quibex/wg-agent/internal/config"
	"github.com/quibex/wg-agent/internal/server"
	"github.com/quibex/wg-agent/internal/wireguard"
)

func main() {
	cfg := config.Load()

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	log.Info("Starting wg-agent")

	wgClient, err := wireguard.NewClient()
	if err != nil {
		log.Error("Failed to create WireGuard client", "error", err)
		os.Exit(1)
	}
	defer wgClient.Close()

	srv := server.New(cfg, log, wgClient)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		log.Error("Server error", "error", err)
		os.Exit(1)
	case sig := <-sigChan:
		log.Info("Received signal, shutting down", "signal", sig)
		if err := srv.Stop(); err != nil {
			log.Error("Server stop error", "error", err)
		}
	}

	log.Info("wg-agent stopped")
}
