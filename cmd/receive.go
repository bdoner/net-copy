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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/bdoner/net-copy/ncproto"
	"github.com/bdoner/net-copy/ncproto/ncserver"

	"github.com/spf13/cobra"
)

var (
	rconf      ncproto.Config
	knownFiles map[uuid.UUID]ncproto.File
)

// receiveCmd represents the receive command
var receiveCmd = &cobra.Command{
	Use:   "receive",
	Short: "Set net-copy to receive files",
	Long: `
	Receive opens a port (optionally given by -p) and starts listening for
	an incoming connection. Once the connection is established net-copy
	receives all the files defined by the sender and closes the connection.`,
	PreRun: func(cmd *cobra.Command, args []string) {
		setupWorkingDir(cmd, args)

		_, err := os.Open(conf.WorkingDirectory)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Printf("Output directory does not exists. creating %s\n", conf.WorkingDirectory)
				err := os.MkdirAll(conf.WorkingDirectory, 0775)
				if err != nil {
					fmt.Fprintf(os.Stderr, "could not create output directory: %v\n", err)
					os.Exit(-1)
				}
			} else {
				fmt.Fprintf(os.Stderr, "could not open output directory: %v\n", err)
				os.Exit(-1)
			}
		}

		wdFiles, err := ioutil.ReadDir(conf.WorkingDirectory)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not open output directory: %v\n", err)
			os.Exit(-1)
		}

		if 0 < len(wdFiles) {
			fmt.Fprintf(os.Stderr, "can only output into an empty directory\n%s is not empty\n", conf.WorkingDirectory)
			os.Exit(-1)
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		srv, err := ncserver.Create(conf.Port)
		if err != nil {
			return err
		}

		defer srv.Connection.Close()

		var cConf ncproto.IMessageType
		err = srv.GetNextMessage(&cConf)
		if err != nil {
			return err
		}

		var c ncproto.Config
		var ok bool
		if c, ok = cConf.(ncproto.Config); !ok {
			return fmt.Errorf("initial message was not of type config")
		}

		conf.Merge(c)
		fmt.Printf("Accepted connection from %s\n", srv.Connection.RemoteAddr().String())
		return loop(srv)
	},
}

func loop(srv *ncserver.Server) error {
	knownFiles = make(map[uuid.UUID]ncproto.File)

	for {
		var message ncproto.IMessageType
		err := srv.GetNextMessage(&message)
		if err != nil {
			return err
		}

		switch message.(type) {
		case ncproto.ConnectionClose:
			cc := message.(ncproto.ConnectionClose)

			if cc.ConnectionID != conf.ConnectionID {
				fmt.Fprintf(os.Stderr, "got close message from %s but expected it from %s\n", cc.ConnectionID.String(), conf.ConnectionID.String())
				continue
			}
			fmt.Println("client says done. closing connection.")
			srv.Connection.Close()
			os.Exit(0)

		case ncproto.FileChunk:
			chunk := message.(ncproto.FileChunk)
			if chunk.ConnectionID != conf.ConnectionID {
				fmt.Fprintf(os.Stderr, "got file chunk from %s but expected it from someone else\n", conf.ConnectionID.String())
				continue
			}
			file, found := knownFiles[chunk.ID]
			if !found {
				return fmt.Errorf("unknown file chunk %v", chunk)
			}

			file.ChunkQueue <- chunk

		case ncproto.File:
			file := message.(ncproto.File)

			if file.ConnectionID != conf.ConnectionID {
				fmt.Fprintf(os.Stderr, "got close message from %s but expected it from %s\n", file.ConnectionID.String(), conf.ConnectionID.String())
				continue
			}
			fmt.Printf("%s (%s)\n", filepath.Join(file.RelativePath, file.Name), file.PrettySize())

			err = os.MkdirAll(filepath.Dir(file.FullFilePath(&conf)), 0775)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
			}

			fd, err := os.OpenFile(file.FullFilePath(&conf), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0775)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
			}

			file.FileDescriptor = fd
			file.ChunkQueue = make(chan ncproto.FileChunk, file.FileSize/int64(conf.ReadBufferSize))
			file.Complete = make(chan bool, 1)
			knownFiles[file.ID] = file

			go func(iFile *ncproto.File) {
				for {

					chunk := <-iFile.ChunkQueue

					n, err := iFile.FileDescriptor.Write(chunk.Data)
					if err != nil {
						fmt.Fprintf(os.Stderr, "error writing chunk %d to file %s: %v\n", chunk.Seq, iFile.RelativeFilePath(&conf), err)
						return
					}

					if n != len(chunk.Data) {
						fmt.Fprintf(os.Stderr, "expected to write %d bytes but wrote %d bytes\n", len(chunk.Data), n)
						return
					}

					if chunk.Seq >= int(iFile.FileSize/int64(conf.ReadBufferSize)) {
						file.Complete <- true
						break
					}
				}
			}(&file)

		// lastPercentage := 0
		// var receivedChunk ncproto.FileChunk
		// for c := int(0); c <= chunks; c++ {
		// 	err := srv.GetNextMessage(&receivedChunk)
		// 	if err != nil {
		// 		fmt.Fprintf(os.Stderr, "error reading chunk %d of %d\n", c, chunks)
		// 		continue
		// 	}

		// }
		// fmt.Printf("\r%s>\n", strings.Repeat("#", 25))
		case ncproto.FileComplete:
			completeMsg := message.(ncproto.FileComplete)
			file := knownFiles[completeMsg.ID]
			<-file.Complete
			close(file.ChunkQueue)
			close(file.Complete)
			file.FileDescriptor.Close()

		case ncproto.Config:
			fmt.Fprintf(os.Stderr, "initial MsgConfig already received.\n")
			continue
		}
	}
}

func init() {
	rootCmd.AddCommand(receiveCmd)

	receiveCmd.Flags().Uint16VarP(&conf.Port, "port", "p", 0, "set the port to listen to. If not set a random, available port is selected")
	receiveCmd.Flags().StringVarP(&conf.WorkingDirectory, "working-dir", "d", ".", "set the directory to output files to")

}
