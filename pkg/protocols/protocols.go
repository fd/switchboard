package protocols

//go:generate gostringer -type=Protocol
type Protocol uint8

const (
	TCP Protocol = 1 + iota
	UDP
	end
)

func (p Protocol) Valid() bool {
	return p > 0 && p < end
}

func (p Protocol) String() string {
	switch p {
	case TCP:
		return "TCP"
	case UDP:
		return "UDP"
	default:
		return "Invalid"
	}
}
