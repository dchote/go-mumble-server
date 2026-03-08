package server

import (
	"context"

	"github.com/dchote/go-mumble-server/internal/config"
)

// Server represents the Mumble server (virtual server or Meta).
type Server struct {
	cfg *config.Config
}

// New creates a new Server.
func New(cfg *config.Config) *Server {
	return &Server{cfg: cfg}
}

// Start begins accepting Mumble and REST connections.
func (s *Server) Start(ctx context.Context) error {
	// TODO: start TCP/TLS, UDP, and REST listeners
	return nil
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	// TODO: close listeners, drain connections
	return nil
}
