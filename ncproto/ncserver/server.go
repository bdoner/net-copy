package ncserver

import (
	"encoding/gob"
	"fmt"
	"net"

	"github.com/bdoner/net-copy/ncproto"
)

// Server struct
type Server struct {
	Connection net.Conn
	Decoder    *gob.Decoder
}

// Create returns a new Server struct with an open, listening connection
func Create(port uint16) (*Server, error) {
	l, err := net.Listen("tcp4", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
		//fmt.Fprintf(os.Stderr, "netcopy/receive: could not listen on port %d\n", conf.Port)
		//os.Exit(-1)
	}

	fmt.Printf("Listening on %s\n", l.Addr().String())
	//fmt.Printf("Outputting files to %s\n", conf.WorkingDirectory)
	conn, err := l.Accept()
	if err != nil {
		return nil, err
		//fmt.Fprintf(os.Stderr, "netcopy/receive: could not accept initial connection\n")
		//os.Exit(-1)
	}

	gob.Register(ncproto.Config{})
	gob.Register(ncproto.File{})
	gob.Register(ncproto.FileChunk{})
	gob.Register(ncproto.FileComplete{})
	gob.Register(ncproto.ConnectionClose{})

	s := Server{
		Connection: conn,
		Decoder:    gob.NewDecoder(conn),
	}

	return &s, nil
}

// GetNextMessage gob decodes the next available message sent by the client
func (s *Server) GetNextMessage(v interface{}) error {
	return s.Decoder.Decode(v)
}
