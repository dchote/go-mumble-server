package transport

import (
	"context"
	"net"
)

// TCPListener accepts Mumble TLS connections on the control port.
func TCPListener(ctx context.Context, addr string) (net.Listener, error) {
	// TODO: implement TLS listener on Mumble port
	return nil, nil
}
