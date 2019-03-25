package ncclient

import (
	"encoding/gob"
	"fmt"
	"net"

	"github.com/bdoner/net-copy/ncproto"
)

// Client that connects to a server
type Client struct {
	Connection net.Conn
	Encoder    *gob.Encoder
}

// Connect to a listening server
func Connect(host string, port uint16) (*Client, error) {
	connAddr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.Dial("tcp", connAddr)
	if err != nil {
		return nil, err
	}

	enc := gob.NewEncoder(conn)

	gob.Register(ncproto.Config{})
	gob.Register(ncproto.File{})
	gob.Register(ncproto.FileChunk{})
	gob.Register(ncproto.ConnectionClose{})

	client := Client{
		Connection: conn,
		Encoder:    enc,
	}

	return &client, nil
}

// SendMessage sends a gob encoded message
func (c *Client) SendMessage(msg ncproto.IMessageType) error {
	return c.Encoder.Encode(&msg)
}
