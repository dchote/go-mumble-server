package transport

import (
	"context"
	"crypto/tls"
	"net"
	"strings"
)

// TCPListener creates a TLS listener for Mumble TCP control connections.
// certPEM and keyPEM may be nil to use a generated self-signed cert.
// securityMode: "legacy" (TLS 1.2+, client cert optional) or "secure" (TLS 1.3 only, client cert required).
func TCPListener(ctx context.Context, addr string, certPEM, keyPEM []byte, securityMode string) (net.Listener, error) {
	if certPEM == nil || keyPEM == nil {
		var err error
		certPEM, keyPEM, err = LoadOrGenerateCert("", "")
		if err != nil {
			return nil, err
		}
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	cfg := buildTLSConfig(cert, securityMode)
	lc := net.ListenConfig{}
	raw, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	return tls.NewListener(raw, cfg), nil
}

func buildTLSConfig(cert tls.Certificate, securityMode string) *tls.Config {
	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	switch strings.ToLower(securityMode) {
	case "secure":
		cfg.MinVersion = tls.VersionTLS13
		cfg.MaxVersion = tls.VersionTLS13
		cfg.ClientAuth = tls.RequireAnyClientCert
	default:
		cfg.MinVersion = tls.VersionTLS12
		cfg.ClientAuth = tls.RequestClientCert
	}
	return cfg
}