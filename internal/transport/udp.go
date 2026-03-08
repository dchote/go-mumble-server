package transport

import (
	"context"
	"net"
)

// UDPListener listens for voice packets on the Mumble port.
func UDPListener(ctx context.Context, addr string) (*net.UDPConn, error) {
	// TODO: implement UDP listener (same port as TCP per protocol)
	return nil, nil
}
