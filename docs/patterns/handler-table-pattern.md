# Handler Table Pattern

> **Status:** Implemented

## Overview

Mumble protocol messages are dispatched using a handler table — an array of handler functions indexed by the message's 16-bit type ID. This provides O(1) dispatch with zero allocation, matching the approach used in both the original Murmur server and the gumble Go client.

The handler table infrastructure lives in the protocol library (`pkg/mumble/protocol`), making it available to both the server and any client implementations built on the library.

## Library-Level Infrastructure

The protocol library defines message type constants and the dispatch mechanism. Handlers are registered by the consumer (server or client), not by the library itself.

```go
package protocol

const (
    MessageVersion                 uint16 = 0
    MessageUDPTunnel               uint16 = 1
    MessageAuthenticate            uint16 = 2
    MessagePing                    uint16 = 3
    MessageReject                  uint16 = 4
    MessageServerSync              uint16 = 5
    MessageChannelRemove           uint16 = 6
    MessageChannelState            uint16 = 7
    MessageUserRemove              uint16 = 8
    MessageUserState               uint16 = 9
    MessageBanList                 uint16 = 10
    MessageTextMessage             uint16 = 11
    MessagePermissionDenied        uint16 = 12
    MessageACL                     uint16 = 13
    MessageQueryUsers              uint16 = 14
    MessageCryptSetup              uint16 = 15
    MessageContextActionModify     uint16 = 16
    MessageContextAction           uint16 = 17
    MessageUserList                uint16 = 18
    MessageVoiceTarget             uint16 = 19
    MessagePermissionQuery         uint16 = 20
    MessageCodecVersion            uint16 = 21
    MessageUserStats               uint16 = 22
    MessageRequestBlob             uint16 = 23
    MessageServerConfig            uint16 = 24
    MessageSuggestConfig           uint16 = 25
    MessagePluginDataTransmission  uint16 = 26
    MessageCount                   uint16 = 27
)

// MessageHandler receives message type, raw payload, and context (e.g., connection).
// Use protocol.ReadMessage to unmarshal payload into native message types.
type MessageHandler func(msgType MessageType, payload []byte, ctx interface{}) error

// HandlerTable is a slice of handlers indexed by message type. Nil entries are no-ops.
type HandlerTable []MessageHandler

func NewHandlerTable() HandlerTable {
    return make(HandlerTable, MessageCount)
}

func (t HandlerTable) Dispatch(msgType MessageType, payload []byte, ctx interface{}) error {
    if int(msgType) >= len(t) || t[msgType] == nil {
        return nil
    }
    return t[msgType](msgType, payload, ctx)
}
```

## Server Handler Table

The server registers handlers for messages it expects to receive from clients. Client-originated messages like `Authenticate`, `UserState` (requests), and `VoiceTarget` get real handlers. Server-originated messages like `Reject`, `ServerSync`, and `ServerConfig` are either nil or return a protocol error.

```go
func (s *Server) buildHandlerTable(client *ServerClient) protocol.HandlerTable {
    var ht protocol.HandlerTable
    ht[protocol.MessageVersion]       = client.handleVersion
    ht[protocol.MessageUDPTunnel]     = client.handleUDPTunnel
    ht[protocol.MessageAuthenticate]  = client.handleAuthenticate
    ht[protocol.MessagePing]          = client.handlePing
    ht[protocol.MessageChannelRemove] = client.handleChannelRemove
    ht[protocol.MessageChannelState]  = client.handleChannelState
    ht[protocol.MessageUserRemove]    = client.handleUserRemove
    ht[protocol.MessageUserState]     = client.handleUserState
    ht[protocol.MessageBanList]       = client.handleBanList
    ht[protocol.MessageTextMessage]   = client.handleTextMessage
    ht[protocol.MessageACL]           = client.handleACL
    ht[protocol.MessageQueryUsers]    = client.handleQueryUsers
    ht[protocol.MessageCryptSetup]    = client.handleCryptSetup
    ht[protocol.MessageContextAction] = client.handleContextAction
    ht[protocol.MessageUserList]      = client.handleUserList
    ht[protocol.MessageVoiceTarget]   = client.handleVoiceTarget
    ht[protocol.MessagePermissionQuery] = client.handlePermissionQuery
    ht[protocol.MessageUserStats]     = client.handleUserStats
    ht[protocol.MessageRequestBlob]   = client.handleRequestBlob
    ht[protocol.MessagePluginDataTransmission] = client.handlePluginData
    return ht
}
```

## Client Handler Table

A client built on the library registers handlers for messages it expects to receive from the server. This is the inverse set — `Reject`, `ServerSync`, `ChannelState`, `UserState`, `CodecVersion`, `ServerConfig`, etc.

```go
func (c *Client) buildHandlerTable() protocol.HandlerTable {
    var ht protocol.HandlerTable
    ht[protocol.MessageVersion]           = c.handleVersion
    ht[protocol.MessageUDPTunnel]         = c.handleUDPTunnel
    ht[protocol.MessagePing]              = c.handlePing
    ht[protocol.MessageReject]            = c.handleReject
    ht[protocol.MessageServerSync]        = c.handleServerSync
    ht[protocol.MessageChannelRemove]     = c.handleChannelRemove
    ht[protocol.MessageChannelState]      = c.handleChannelState
    ht[protocol.MessageUserRemove]        = c.handleUserRemove
    ht[protocol.MessageUserState]         = c.handleUserState
    ht[protocol.MessageBanList]           = c.handleBanList
    ht[protocol.MessageTextMessage]       = c.handleTextMessage
    ht[protocol.MessagePermissionDenied]  = c.handlePermissionDenied
    ht[protocol.MessageACL]               = c.handleACL
    ht[protocol.MessageCryptSetup]        = c.handleCryptSetup
    ht[protocol.MessageContextActionModify] = c.handleContextActionModify
    ht[protocol.MessagePermissionQuery]   = c.handlePermissionQuery
    ht[protocol.MessageCodecVersion]      = c.handleCodecVersion
    ht[protocol.MessageUserStats]         = c.handleUserStats
    ht[protocol.MessageServerConfig]      = c.handleServerConfig
    ht[protocol.MessageSuggestConfig]     = c.handleSuggestConfig
    ht[protocol.MessagePluginDataTransmission] = c.handlePluginData
    return ht
}
```

## Packet Framing

Packet framing is provided by the library (`pkg/mumble/protocol`). Both server and client use the same read/write functions.

Every TCP packet has a 6-byte header:

```
┌──────────────────┬──────────────────────────┐
│  Type (uint16)   │  Payload Length (uint32)  │
│  2 bytes, BE     │  4 bytes, BE             │
├──────────────────┴──────────────────────────┤
│  Message payload (variable length, native Go wire format) │
└─────────────────────────────────────────────┘
```

The library provides:

```go
package protocol

func ReadPacket(r io.Reader) (msgType MessageType, payload []byte, err error)
func WritePacket(w io.Writer, msgType MessageType, payload []byte) error
func WriteMessage(w io.Writer, msgType MessageType, msg Message) error  // marshals via wire encoder
```

The read loop (identical for server and client):

1. Call `protocol.ReadPacket(r)` — reads 6-byte header and payload
2. Call `table.Dispatch(msgType, payload, ctx)` — dispatches to registered handler
3. Repeat

## Message Direction Summary

| Type | Message | Server Handles | Client Handles |
|------|---------|:--------------:|:--------------:|
| 0 | Version | yes | yes |
| 1 | UDPTunnel | yes | yes |
| 2 | Authenticate | yes | — |
| 3 | Ping | yes | yes |
| 4 | Reject | — | yes |
| 5 | ServerSync | — | yes |
| 6 | ChannelRemove | yes | yes |
| 7 | ChannelState | yes | yes |
| 8 | UserRemove | yes | yes |
| 9 | UserState | yes | yes |
| 10 | BanList | yes | yes |
| 11 | TextMessage | yes | yes |
| 12 | PermissionDenied | — | yes |
| 13 | ACL | yes | yes |
| 14 | QueryUsers | yes | yes |
| 15 | CryptSetup | yes | yes |
| 16 | ContextActionModify | — | yes |
| 17 | ContextAction | yes | — |
| 18 | UserList | yes | yes |
| 19 | VoiceTarget | yes | — |
| 20 | PermissionQuery | yes | yes |
| 21 | CodecVersion | — | yes |
| 22 | UserStats | yes | yes |
| 23 | RequestBlob | yes | — |
| 24 | ServerConfig | — | yes |
| 25 | SuggestConfig | — | yes |
| 26 | PluginDataTransmission | yes | yes |

## Error Handling

Handler errors fall into two categories:

- **Protocol errors** — Invalid message, wrong state, malformed data. Log and optionally disconnect.
- **Permission errors** (server-side) — Client lacks permission for the requested action. Send `PermissionDenied` and continue.

## Reference

- gumble handler table: `research/gumble/gumble/handlers.go`
- Murmur dispatch: `research/mumble/src/murmur/Messages.cpp`
