package discovery

import (
	"context"
	"log/slog"

	"github.com/grandcat/zeroconf"
)

const serviceType = "_mumble._tcp"
const localDomain = "local."

// Server advertises the Mumble server via mDNS/Bonjour for LAN discovery.
type Server struct {
	server *zeroconf.Server
	name   string
	port   int
}

// NewServer creates an mDNS advertiser. Call Start to begin advertising.
func NewServer(name string, port int) *Server {
	if name == "" {
		name = "go-mumble-server"
	}
	return &Server{name: name, port: port}
}

// Start begins mDNS advertisement. Blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	server, err := zeroconf.Register(s.name, serviceType, localDomain, s.port, []string{"txtvers=1"}, nil)
	if err != nil {
		return err
	}
	s.server = server
	slog.Info("mDNS discovery started", "name", s.name, "port", s.port, "service", serviceType)
	<-ctx.Done()
	return s.Shutdown()
}

// Shutdown stops mDNS advertisement.
func (s *Server) Shutdown() error {
	if s.server != nil {
		s.server.Shutdown()
		s.server = nil
		slog.Info("mDNS discovery stopped", "name", s.name)
	}
	return nil
}
