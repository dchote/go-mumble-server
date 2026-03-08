package mumble

// BanEntry represents a server ban (IP or certificate hash).
type BanEntry struct {
	Address  []byte
	Mask     uint32
	Name     string
	Hash     string
	Reason   string
	Start    string
	Duration uint32
}
