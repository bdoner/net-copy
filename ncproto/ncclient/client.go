package ncclient

import (
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	"github.com/bdoner/net-copy/ncproto"
)

// Client that connects to a server
type Client struct {
	Connection net.Conn
	Encoder    *gob.Encoder
	Decoder    *gob.Decoder
}

// Connect to a listening server
func Connect(host string, port uint16) (*Client, error) {
	connAddr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.Dial("tcp", connAddr)
	if err != nil {
		return nil, err
	}

	c := getClient(conn)
	return c, nil
}

// Listen returns a new Server struct with an open, listening connection
func Listen(port uint16) (*Client, error) {
	l, err := net.Listen("tcp4", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
		//fmt.Fprintf(os.Stderr, "netcopy/receive: could not listen on port %d\n", conf.Port)
		//os.Exit(-1)
	}

	fmt.Printf("Listening on %s\n", l.Addr().String())
	conn, err := l.Accept()
	if err != nil {
		return nil, err
	}

	c := getClient(conn)
	return c, nil
}

func getClient(conn net.Conn) *Client {
	gob.Register(ncproto.Config{})
	gob.Register(ncproto.File{})
	gob.Register(ncproto.FileChunk{})
	gob.Register(ncproto.FileComplete{})
	gob.Register(ncproto.ConnectionClose{})

	c := Client{
		Connection: conn,
		Decoder:    gob.NewDecoder(conn),
		Encoder:    gob.NewEncoder(conn),
	}

	return &c
}

// GetNextMessage gob decodes the next available message sent by the client
func (c *Client) GetNextMessage(v interface{}) error {
	return c.Decoder.Decode(v)
}

// SendMessage sends a gob encoded message
func (c *Client) SendMessage(msg ncproto.INetCopyMessage) error {
	return c.Encoder.Encode(&msg)
}

// SendFile will send an entire File to the server
func (c *Client) SendFile(file *ncproto.File, wg *sync.WaitGroup, conf *ncproto.Config) {
	wg.Add(1)
	defer wg.Done()

	if !conf.Quiet {
		fmt.Printf("%s (%s)\n", file.RelativeFilePath(conf), file.PrettySize())
	}

	fp, err := os.Open(file.FullFilePath(conf))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening file %s\n", file.RelativeFilePath(conf))
	}

	c.SendMessage(file)

	readBuffer := make([]byte, conf.ReadBufferSize)
	sentChunks := 0
	//lastPercentage := 0
	for {
		n, err := fp.Read(readBuffer)
		if n == 0 && err == io.EOF {
			break
		}

		if err != nil && err != io.EOF {
			fmt.Fprintf(os.Stderr, "error reading file %s\n", file.RelativeFilePath(conf))
			break
		}

		fchunk := ncproto.FileChunk{
			ID:           file.ID,
			ConnectionID: conf.ConnectionID,
			Data:         readBuffer[:n],
			Seq:          sentChunks,
		}

		// bar, progress := file.GetProgress(sentChunks, 25, &conf)
		// if lastPercentage < progress {
		// 	fmt.Printf("\r%s", bar)
		// 	lastPercentage = progress
		// }

		sentChunks++
		c.SendMessage(fchunk)
		//enc.Encode(fchunk)
	}

	c.SendMessage(ncproto.FileComplete{ConnectionID: conf.ConnectionID, ID: file.ID})

	//fmt.Printf("\r%s>\n", strings.Repeat("#", 25))
}
