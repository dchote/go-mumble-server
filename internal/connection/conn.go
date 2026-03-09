package connection

import (
	"context"
	"crypto/sha1"
	"crypto/tls"
	"encoding/hex"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/dchote/go-mumble-server/pkg/mumble/crypto"
	"github.com/dchote/go-mumble-server/pkg/mumble/protocol"
	"github.com/dchote/go-mumble-server/pkg/mumble/protocol/messages"
)

// State is the connection state.
type State int

const (
	StateAuthenticating State = iota
	StateActive
	StateClosed
)

// Conn represents a Mumble client connection.
type Conn struct {
	net.Conn
	Crypt *crypto.CryptState

	mu        sync.RWMutex
	state     State
	sessionID uint32
	serverID  uint
	user      *struct {
		name      string
		userID    uint32
		channelID uint32
	}

	writeCh chan writeReq
	onClose func(*Conn)
}

type writeReq struct {
	msgType protocol.MessageType
	msg     messages.Message
	payload []byte
}

// New creates a new connection.
func New(c net.Conn, crypt *crypto.CryptState, onClose func(*Conn)) *Conn {
	return &Conn{
		Conn:    c,
		Crypt:   crypt,
		state:   StateAuthenticating,
		writeCh: make(chan writeReq, 64),
		onClose: onClose,
	}
}

// SessionID returns the session ID (0 until authenticated).
func (c *Conn) SessionID() uint32 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionID
}

// SetSessionID sets the session ID.
func (c *Conn) SetSessionID(id uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionID = id
}

// State returns the connection state.
func (c *Conn) State() State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// SetActive sets state to Active.
func (c *Conn) SetActive() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = StateActive
}

// SetUser sets the user info.
func (c *Conn) SetUser(name string, userID uint32, channelID uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.user == nil {
		c.user = &struct {
			name      string
			userID    uint32
			channelID uint32
		}{}
	}
	c.user.name = name
	c.user.userID = userID
	c.user.channelID = channelID
}

// CertificateHash returns the SHA-1 hex fingerprint of the client's TLS certificate, or empty if unavailable.
func (c *Conn) CertificateHash() string {
	tlsConn, ok := c.Conn.(*tls.Conn)
	if !ok {
		return ""
	}
	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return ""
	}
	der := state.PeerCertificates[0].Raw
	hash := sha1.Sum(der)
	return strings.ToLower(hex.EncodeToString(hash[:]))
}

// UserName returns the username.
func (c *Conn) UserName() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.user == nil {
		return ""
	}
	return c.user.name
}

// UserChannel returns the user's channel ID.
func (c *Conn) UserChannel() uint32 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.user == nil {
		return 0
	}
	return c.user.channelID
}

// WriteMessage queues a message for sending.
func (c *Conn) WriteMessage(msgType protocol.MessageType, msg messages.Message) error {
	select {
	case c.writeCh <- writeReq{msgType: msgType, msg: msg}:
		return nil
	default:
		return io.ErrShortWrite
	}
}

// WriteRaw queues raw payload for sending.
func (c *Conn) WriteRaw(msgType protocol.MessageType, payload []byte) error {
	select {
	case c.writeCh <- writeReq{msgType: msgType, payload: payload}:
		return nil
	default:
		return io.ErrShortWrite
	}
}

// Close closes the connection and notifies onClose.
func (c *Conn) Close() error {
	c.mu.Lock()
	if c.state == StateClosed {
		c.mu.Unlock()
		return nil
	}
	c.state = StateClosed
	onClose := c.onClose
	c.mu.Unlock()
	close(c.writeCh)
	err := c.Conn.Close()
	if onClose != nil {
		onClose(c)
	}
	return err
}

// Run runs the read and write loops.
func (c *Conn) Run(ctx context.Context, handler protocol.HandlerTable) error {
	defer c.Close()
	go c.writeLoop()

	for {
		c.SetReadDeadline(time.Now().Add(30 * time.Second))
		msgType, payload, err := protocol.ReadPacket(c.Conn)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		c.mu.RLock()
		state := c.state
		c.mu.RUnlock()
		if state == StateAuthenticating && msgType != protocol.MessageVersion && msgType != protocol.MessageAuthenticate {
			continue
		}
		if err := handler.Dispatch(msgType, payload, c); err != nil {
			slog.Debug("handler error", "type", msgType, "err", err)
		}
	}
}

func (c *Conn) writeLoop() {
	for req := range c.writeCh {
		if len(req.payload) > 0 {
			protocol.WritePacket(c, req.msgType, req.payload)
		} else {
			protocol.WriteMessage(c, req.msgType, req.msg)
		}
	}
}
