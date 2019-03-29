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
	gob.Register(ncproto.FileComplete{})
	gob.Register(ncproto.ConnectionClose{})

	client := Client{
		Connection: conn,
		Encoder:    enc,
	}

	return &client, nil
}

// SendMessage sends a gob encoded message
func (c *Client) SendMessage(msg ncproto.INetCopyMessage) error {
	return c.Encoder.Encode(&msg)
}

// SendFile will send an entire File to the server
func (c *Client) SendFile(file *ncproto.File, wg *sync.WaitGroup, conf *ncproto.Config) {

	defer wg.Done()

	if !conf.Quiet {
		fmt.Printf("%s (%s)\n", file.RelativeFilePath(conf), file.PrettySize())
	}

	fp, err := os.Open(file.FullFilePath(conf))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening file %s", file.RelativeFilePath(conf))
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
			fmt.Fprintf(os.Stderr, "error reading file %s", file.RelativeFilePath(conf))
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
