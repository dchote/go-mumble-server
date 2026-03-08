package transport

import (
	"context"
	"net"
)

// UDPListener listens for voice packets on the Mumble port.
// TCP and UDP share the same port per Mumble spec; they use different sockets so both can bind.
func UDPListener(ctx context.Context, addr string) (*net.UDPConn, error) {
	lc := net.ListenConfig{}
	conn, err := lc.ListenPacket(ctx, "udp", addr)
	if err != nil {
		return nil, err
	}
	udp, ok := conn.(*net.UDPConn)
	if !ok {
		conn.Close()
		return nil, nil
	}
	return udp, nil
}
