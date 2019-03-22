// Copyright © 2019 Bdoner
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cmd

import (
	"encoding/gob"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"path/filepath"

	"github.com/bdoner/net-copy/ncproto"
	"github.com/spf13/cobra"
)

var rconf ncproto.Config

// receiveCmd represents the receive command
var receiveCmd = &cobra.Command{
	Use:   "receive",
	Short: "Set net-copy to receive files",
	Long: `
	Receive opens a port (optionally given by -p) and starts listening for
	an incoming connection. Once the connection is established net-copy
	receives all the files defined by the sender and closes the connection.

	If -t is provided the lowest value between the sender and receiver is used.`,
	PreRun: setupConfig,
	Run: func(cmd *cobra.Command, args []string) {
		conn := getConnection()
		defer conn.Close()

		var messageType ncproto.MessageType
		dec := gob.NewDecoder(conn)
		err := dec.Decode(&messageType)
		if err != nil {
			log.Fatalf("net-copy/receive: error decoding Message: %v", err)
		}

		if messageType == ncproto.MsgConfig {
			var cConf ncproto.Config
			err := dec.Decode(&cConf)
			if err != nil {
				log.Fatalf("net-copy/receive: error decoding Message.Data: %v", err)
			}
			conf.Merge(cConf)
			fmt.Printf("Accepted connection from %s\n", conn.RemoteAddr().String())

		} else {
			log.Fatal("First message has to be of type MsgConfigure\n")
		}

		loop(dec, &conn)

	},
}

func loop(dec *gob.Decoder, conn *net.Conn) {
	for {
		var messageType ncproto.MessageType
		err := dec.Decode(&messageType)
		if err != nil {
			log.Fatalf("net-copy/receive: error decoding Message: %v", err)
		}

		switch messageType {
		case ncproto.MsgClose:
			var cc ncproto.ConnectionClose
			err := dec.Decode(&cc)
			if err != nil {
				fmt.Fprintf(os.Stderr, "net-copy/receive: error decoding close message. %v\n", err)
			}
			if cc.ConnectionID != conf.ConnectionID {
				fmt.Fprintf(os.Stderr, "net-copy/receive: got close message from %s but expected it from %s\n", cc.ConnectionID.String(), conf.ConnectionID.String())
				continue
			}
			fmt.Println("client says done. closing connection.")
			(*conn).Close()
			os.Exit(0)

		case ncproto.MsgFile:
			var file ncproto.File
			err := dec.Decode(&file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "net-copy/receive: error decoding file message. %v\n", err)
			}
			if file.ConnectionID != conf.ConnectionID {
				fmt.Fprintf(os.Stderr, "net-copy/receive: got close message from %s but expected it from %s\n", file.ConnectionID.String(), conf.ConnectionID.String())
				continue
			}
			fmt.Printf("Got file %s of size %s\n", file.FullPath(&conf), file.PrettySize())
			chunks := uint64(math.Ceil(float64(file.FileSize / int64(conf.ReadBufferSize))))

			err = os.MkdirAll(filepath.Dir(file.FullPath(&conf)), 0775)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
			}

			fp, err := os.Create(file.FullPath(&conf))
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
			}

			var receivedChunk ncproto.FileChunk
			for c := uint64(0); c <= chunks; c++ {
				err := dec.Decode(&receivedChunk)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error reading chunk %d of %d\n", c, chunks)
				}
				if receivedChunk.ConnectionID != conf.ConnectionID {
					fmt.Fprintf(os.Stderr, "got file chunk from %s but expected it from %s\n", file.ConnectionID.String(), conf.ConnectionID.String())
					continue
				}

				n, err := fp.Write(receivedChunk.Data)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error writing chunk %d to file %s: %v\n", c, filepath.Join(file.RelativePath, file.Name), err)
					continue
				}

				if n != len(receivedChunk.Data) {
					fmt.Fprintf(os.Stderr, "expected to write %d bytes but wrote %d bytes\n", len(receivedChunk.Data), n)
					continue

				}
			}

		case ncproto.MsgConfig:
			fmt.Fprintf(os.Stderr, "net-copy/receive: initial MsgConfig already received.\n")
			continue
		}
	}
}

func getConnection() net.Conn {
	l, err := net.Listen("tcp4", fmt.Sprintf(":%d", conf.Port))
	if err != nil {
		log.Fatalf("netcopy/receive: could not listen on port %d\n", conf.Port)
	}

	fmt.Printf("Listening on %s\n", l.Addr().String())
	fmt.Printf("Outputting files to %s\n", conf.WorkingDirectory)
	conn, err := l.Accept()
	if err != nil {
		log.Fatalf("netcopy/receive: could not accept initial connection\n")
	}

	return conn
}

func init() {
	rootCmd.AddCommand(receiveCmd)

	receiveCmd.Flags().Uint16VarP(&conf.Port, "port", "p", 0, "set the port to listen to. If not set a random, available port is selected")
	receiveCmd.Flags().StringVarP(&conf.WorkingDirectory, "working-dir", "d", ".", "set the directory to output files to")
	receiveCmd.Flags().Uint16VarP(&conf.Threads, "threads", "t", 1, "define how many concurrent transfers to run")

}
