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
