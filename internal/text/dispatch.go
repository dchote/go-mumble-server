package text

import "github.com/dchote/go-mumble-server/pkg/mumble"

// Dispatch sends a text message to the appropriate recipients.
func Dispatch(msg *mumble.TextMessage) error {
	// TODO: implement channel/private/tree routing
	return nil
}
