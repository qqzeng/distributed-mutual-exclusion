package netq2

// Server defines the interface for a server.
type TcpPeer interface {
	BuildAllConn() (map[int]int, error)

	ReadData() (int, []byte, error)

	WriteData(connID int, data []byte) error

	CloseConn(connID int) error

	Close() error
}
