package transport

import (
	"context"
	"crypto/tls"
	"net"
)

// TCPListener creates a TLS listener for Mumble TCP control connections.
// certPEM and keyPEM may be nil to use a generated self-signed cert.
// Uses permissive TLS (TLS 1.2+, optional client certs); per-client security
// is negotiated via the Version message exchange.
func TCPListener(ctx context.Context, addr string, certPEM, keyPEM []byte) (net.Listener, error) {
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
	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequestClientCert,
	}
	lc := net.ListenConfig{}
	raw, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	return tls.NewListener(raw, cfg), nil
}