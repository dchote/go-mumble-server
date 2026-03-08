package mumble

// Version encodes/decodes Mumble protocol version (major.minor.patch).
// Major and minor form the protocol version; patch is implementation-specific.
type Version struct {
	Major uint32
	Minor uint32
	Patch uint32
}
