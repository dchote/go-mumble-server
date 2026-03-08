package acl

import "github.com/dchote/go-mumble-server/pkg/mumble"

// Evaluator resolves permissions for a user in a channel.
type Evaluator struct{}

// Check returns whether the user has the given permission in the channel.
func (e *Evaluator) Check(userID uint32, channelID uint32, perm mumble.Permission) bool {
	// TODO: implement ACL evaluation; see docs/patterns/acl-evaluation-pattern.md
	return false
}
