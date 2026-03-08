package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dchote/go-mumble-server/internal/config"
	"github.com/dchote/go-mumble-server/internal/connection"
	"github.com/dchote/go-mumble-server/internal/discovery"
	"github.com/dchote/go-mumble-server/internal/mumble"
	"github.com/dchote/go-mumble-server/internal/rest"
	"github.com/dchote/go-mumble-server/internal/transport"
	"github.com/dchote/go-mumble-server/pkg/mumble/crypto"
	"github.com/dchote/go-mumble-server/pkg/mumble/protocol"
	"github.com/dchote/go-mumble-server/pkg/mumble/protocol/messages"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

// Server represents the Mumble server (virtual server or Meta).
type Server struct {
	cfg    *config.Config
	db     *gorm.DB
	feFS   fs.FS
	mu     sync.Mutex
	http   *http.Server
	tcpLn  interface{ Close() error }
	udpConn interface{ Close() error }
	mdns   *discovery.Server
}

// New creates a new Server.
func New(cfg *config.Config, db *gorm.DB, feFS fs.FS) *Server {
	return &Server{cfg: cfg, db: db, feFS: feFS}
}

// Start begins accepting Mumble and REST connections.
func (s *Server) Start(ctx context.Context) error {
	var certPEM, keyPEM []byte
	if s.cfg.SSLCertPath != "" && s.cfg.SSLKeyPath != "" {
		var err error
		certPEM, keyPEM, err = transport.LoadOrGenerateCert(s.cfg.SSLCertPath, s.cfg.SSLKeyPath)
		if err != nil {
			return err
		}
	}

	mumbleAddr := formatAddr(s.cfg.Host, s.cfg.MumblePort)
	restAddr := formatAddr(s.cfg.Host, s.cfg.RESTPort)

	tcpLn, err := transport.TCPListener(ctx, mumbleAddr, certPEM, keyPEM, s.cfg.SecurityMode)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.tcpLn = tcpLn
	s.mu.Unlock()
	slog.Info("Mumble TCP/TLS listening", "addr", mumbleAddr)

	udpConn, err := transport.UDPListener(ctx, mumbleAddr)
	if err != nil {
		tcpLn.Close()
		return err
	}
	s.mu.Lock()
	s.udpConn = udpConn
	s.mu.Unlock()
	slog.Info("Mumble UDP listening", "addr", mumbleAddr)

	ms := mumble.NewServer(s.cfg, s.db, 1, udpConn)
	handler := rest.RouterWithMumble(s.db, s.cfg, s.feFS, &rest.MumbleUserAdapter{Manager: ms.UserManager()})
	s.http = &http.Server{
		Addr:    restAddr,
		Handler: handler,
	}
	slog.Info("REST API listening", "addr", restAddr)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})

	g.Go(func() error {
		return s.acceptLoop(gctx, tcpLn, ms)
	})

	g.Go(func() error {
		return s.udpReadLoop(gctx, udpConn, ms)
	})

	if s.cfg.Bonjour {
		name := s.cfg.RegisterName
		if name == "" {
			name = "go-mumble-server"
		}
		s.mdns = discovery.NewServer(name, s.cfg.MumblePort)
		g.Go(func() error {
			if err := s.mdns.Start(gctx); err != nil {
				slog.Warn("mDNS discovery failed (continuing without)", "err", err)
				return nil // non-fatal: server continues without LAN discovery
			}
			return nil
		})
	}

	return g.Wait()
}

func formatAddr(host string, port int) string {
	if host == "" {
		host = "0.0.0.0"
	}
	return fmt.Sprintf("%s:%d", host, port)
}

func (s *Server) acceptLoop(ctx context.Context, ln net.Listener, ms *mumble.Server) error {
	mode := crypto.ModeLegacy
	if strings.ToLower(s.cfg.SecurityMode) == "secure" {
		mode = crypto.ModeSecure
	}
	for {
		raw, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				return nil
			}
			return err
		}
		crypt := crypto.NewCryptState(mode)
		remoteAddr := raw.RemoteAddr().String()
		slog.Info("Mumble client connected", "remote", remoteAddr)
		conn := connection.New(raw, crypt, func(c *connection.Conn) {
			sid := c.SessionID()
			name := c.UserName()
			u := ms.UserManager().Remove(sid)
			ms.UnregisterConn(sid)
			ms.Broadcast(sid, protocol.MessageUserRemove, &messages.UserRemove{Session: sid, Actor: 0})
			if name != "" || u != nil {
				n := name
				if u != nil {
					n = u.Name
				}
				slog.Info("Mumble client disconnected", "remote", remoteAddr, "session", sid, "user", n)
			} else {
				slog.Info("Mumble client disconnected", "remote", remoteAddr, "session", sid)
			}
		})
		go func() {
			_ = conn.WriteMessage(protocol.MessageVersion, &messages.Version{
				Release: "go-mumble-server",
				OS:      "Go",
				OSVersion: "1.0",
			})
			_ = conn.Run(ctx, ms.HandlerTable())
		}()
	}
}


func (s *Server) udpReadLoop(ctx context.Context, conn net.PacketConn, ms *mumble.Server) error {
	buf := make([]byte, 65535)
	for {
		conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				select {
				case <-ctx.Done():
					return nil
				default:
					continue
				}
			}
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				return nil
			}
			return err
		}
		if n > 0 && ms != nil {
			ms.HandleUDP(addr, buf[:n])
		}
	}
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("Server shutting down")
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.http != nil {
		s.http.Shutdown(ctx)
	}
	if s.tcpLn != nil {
		s.tcpLn.Close()
		s.tcpLn = nil
	}
	if s.udpConn != nil {
		s.udpConn.Close()
		s.udpConn = nil
	}
	if s.mdns != nil {
		s.mdns.Shutdown()
		s.mdns = nil
	}
	return nil
}
