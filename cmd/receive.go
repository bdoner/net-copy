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
	"sync"

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

		if cConf.Type() != ncproto.MsgConfig {
			return fmt.Errorf("initial message was not of type config")
		}

		conf.Merge(cConf.(ncproto.Config))
		fmt.Printf("Accepted connection from %s\n", srv.Connection.RemoteAddr().String())
		return loop(srv)
	},
}

func loop(srv *ncserver.Server) error {
	knownFiles = make(map[uuid.UUID]ncproto.File)
	var writeWg sync.WaitGroup

	for {
		var message ncproto.IMessageType
		err := srv.GetNextMessage(&message)
		if err != nil {
			return err
		}

		switch message.Type() {
		case ncproto.MsgConnectionClose:
			cc := message.(ncproto.ConnectionClose) //.(ncproto.ConnectionClose)

			if cc.ConnectionID != conf.ConnectionID {
				fmt.Fprintf(os.Stderr, "got close message from %s but expected it from %s\n", cc.ConnectionID.String(), conf.ConnectionID.String())
				continue
			}
			fmt.Println("client says done. closing connection.")
			srv.Connection.Close()
			writeWg.Wait()
			os.Exit(0)

		case ncproto.MsgFileChunk:
			chunk := message.(ncproto.FileChunk)
			if chunk.ConnectionID != conf.ConnectionID {
				fmt.Fprintf(os.Stderr, "got file chunk from %s but expected it from someone else\n", conf.ConnectionID.String())
				continue
			}
			file, found := knownFiles[chunk.ID]
			if !found {
				return fmt.Errorf("unknown file chunk %v", chunk)
			}

			writeWg.Add(1)
			go func(file *ncproto.File, chunk *ncproto.FileChunk, wwg *sync.WaitGroup) {

				defer wwg.Done()
				fd, err := os.OpenFile(file.FullFilePath(&conf), os.O_APPEND|os.O_WRONLY, 0775)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%v\n", err)
				}

				defer (func() {
					err := fd.Close()
					if err != nil {
						fmt.Fprintf(os.Stderr, "%v\n", err)
					}
				})()

				n, err := fd.Write(chunk.Data)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error writing chunk %d to file %s: %v\n", chunk.Seq, file.RelativeFilePath(&conf), err)
					return
				}

				if n != len(chunk.Data) {
					fmt.Fprintf(os.Stderr, "expected to write %d bytes but wrote %d bytes\n", len(chunk.Data), n)
					return
				}

			}(&file, &chunk, &writeWg)

		case ncproto.MsgFile:
			file := message.(ncproto.File)

			if file.ConnectionID != conf.ConnectionID {
				fmt.Fprintf(os.Stderr, "got close message from %s but expected it from %s\n", file.ConnectionID.String(), conf.ConnectionID.String())
				continue
			}
			fmt.Printf("%s (%s)\n", filepath.Join(file.RelativePath, file.Name), file.PrettySize())
			//chunks := int(math.Ceil(float64(file.FileSize / int64(conf.ReadBufferSize))))

			err = os.MkdirAll(filepath.Dir(file.FullFilePath(&conf)), 0775)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
			}

			fd, err := os.OpenFile(file.FullFilePath(&conf), os.O_CREATE, 0775)
			defer (func() {
				err := fd.Close()
				if err != nil {
					fmt.Fprintf(os.Stderr, "%v\n", err)
				}
			})()

			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
			}

			knownFiles[file.ID] = file

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

		case ncproto.MsgConfig:
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
