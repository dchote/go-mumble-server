package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/dchote/go-mumble-server/pkg/mumble/crypto"
	"github.com/dchote/go-mumble-server/pkg/mumble/protocol"
	"github.com/dchote/go-mumble-server/pkg/mumble/protocol/messages"
)

// Test client using pkg/mumble to verify channel tree sync. Build with:
//
//	go build -o test-client ./cmd/test-client
//
// Run: ./test-client [addr]
//
// Example: ./test-client localhost:64738
func main() {
	addr := "localhost:64738"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}

	conn, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect error: %s\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	crypt := crypto.NewCryptState(crypto.ModeLegacy)
	channels := make(map[uint32]*channelInfo)
	users := make(map[uint32]*userInfo)
	var sessionID uint32

	// 1. Read server Version
	msgType, _, err := protocol.ReadPacket(conn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read version: %s\n", err)
		os.Exit(1)
	}
	if msgType != protocol.MessageVersion {
		fmt.Fprintf(os.Stderr, "expected Version, got type %d\n", msgType)
		os.Exit(1)
	}

	// 2. Send our Version
	if err := protocol.WriteMessage(conn, protocol.MessageVersion, &messages.Version{
		Release: "test-client",
		OS:      "Go",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "write version: %s\n", err)
		os.Exit(1)
	}

	// 3. Send Authenticate
	if err := protocol.WriteMessage(conn, protocol.MessageAuthenticate, &messages.Authenticate{
		Username: "test-client",
		Opus:     true,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "write authenticate: %s\n", err)
		os.Exit(1)
	}

	// 4. Read until ServerSync
	for {
		msgType, payload, err := protocol.ReadPacket(conn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read: %s\n", err)
			os.Exit(1)
		}

		switch msgType {
		case protocol.MessageReject:
			msg, _ := protocol.ReadMessage(msgType, payload)
			if m, ok := msg.(*messages.Reject); ok {
				fmt.Fprintf(os.Stderr, "rejected: %s\n", m.Reason)
			} else {
				fmt.Fprintf(os.Stderr, "rejected\n")
			}
			os.Exit(1)

		case protocol.MessageCryptSetup:
			msg, err := protocol.ReadMessage(msgType, payload)
			if err != nil {
				fmt.Fprintf(os.Stderr, "crypt setup parse: %s\n", err)
				os.Exit(1)
			}
			cs := msg.(*messages.CryptSetup)
			if err := crypt.SetKey(cs.Key, cs.ClientNonce, cs.ServerNonce); err != nil {
				fmt.Fprintf(os.Stderr, "crypt setkey: %s\n", err)
				os.Exit(1)
			}

		case protocol.MessageChannelState:
			msg, err := protocol.ReadMessage(msgType, payload)
			if err != nil {
				fmt.Fprintf(os.Stderr, "channel state parse: %s\n", err)
				continue
			}
			cs := msg.(*messages.ChannelState)
			ch := &channelInfo{
				ID: cs.ChannelID, Name: cs.Name,
				ParentID: cs.Parent, HasParent: cs.HasParent,
				Position: cs.Position,
			}
			channels[cs.ChannelID] = ch

		case protocol.MessageUserState:
			msg, err := protocol.ReadMessage(msgType, payload)
			if err != nil {
				fmt.Fprintf(os.Stderr, "user state parse: %s\n", err)
				continue
			}
			us := msg.(*messages.UserState)
			users[us.Session] = &userInfo{Session: us.Session, Name: us.Name, ChannelID: us.ChannelID}

		case protocol.MessageServerSync:
			msg, err := protocol.ReadMessage(msgType, payload)
			if err != nil {
				fmt.Fprintf(os.Stderr, "server sync parse: %s\n", err)
				os.Exit(1)
			}
			ss := msg.(*messages.ServerSync)
			sessionID = ss.Session
			goto synced

		case protocol.MessageCodecVersion, protocol.MessageServerConfig:
			// ignore
		default:
			// ignore other message types
		}
	}

synced:
	fmt.Printf("Connected (session %d)\n", sessionID)
	fmt.Printf("Channels (%d total):\n", len(channels))

	// Build and print tree from root
	root := channels[0]
	if root != nil {
		printChannelTree(channels, root, 0)
	} else {
		fmt.Println("  (no root channel)")
	}

	fmt.Printf("\nUsers (%d):\n", len(users))
	userIDs := make([]uint32, 0, len(users))
	for id := range users {
		userIDs = append(userIDs, id)
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })
	for _, id := range userIDs {
		u := users[id]
		chName := "?"
		if ch := channels[u.ChannelID]; ch != nil {
			chName = ch.Name
		}
		fmt.Printf("  [%d] %s in %s\n", id, u.Name, chName)
	}
}

type channelInfo struct {
	ID        uint32
	Name      string
	ParentID  uint32
	HasParent bool
	Position  int32
}

type userInfo struct {
	Session   uint32
	Name      string
	ChannelID uint32
}

func printChannelTree(channels map[uint32]*channelInfo, ch *channelInfo, depth int) {
	indent := strings.Repeat("  ", depth)
	fmt.Printf("%s[%d] %s\n", indent, ch.ID, ch.Name)
	var children []*channelInfo
	for _, c := range channels {
		if c.HasParent && c.ParentID == ch.ID {
			children = append(children, c)
		}
	}
	sort.Slice(children, func(i, j int) bool { return children[i].Position < children[j].Position })
	for _, child := range children {
		printChannelTree(channels, child, depth+1)
	}
}
