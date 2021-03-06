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
	"github.com/bdoner/net-copy/ncproto/ncclient"

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
					fmt.Fprintf(os.Stderr, "PreRun: could not create output directory: %v\n", err)
					os.Exit(-1)
				}
			} else {
				fmt.Fprintf(os.Stderr, "PreRun: could not open output directory: %v\n", err)
				os.Exit(-1)
			}
		}

		wdFiles, err := ioutil.ReadDir(conf.WorkingDirectory)
		if err != nil {
			fmt.Fprintf(os.Stderr, "PreRun: could not open output directory: %v\n", err)
			os.Exit(-1)
		}

		if 0 < len(wdFiles) {
			fmt.Fprintf(os.Stderr, "PreRun: can only output into an empty directory\n%s is not empty\n", conf.WorkingDirectory)
			os.Exit(-1)
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		srv, err := ncclient.Listen(conf.Port)
		if err != nil {
			return err
		}

		defer srv.Connection.Close()

		var cConf ncproto.INetCopyMessage
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

func loop(srv *ncclient.Client) error {
	knownFiles = make(map[uuid.UUID]ncproto.File)
	var fwg sync.WaitGroup

outer:
	for {
		var message ncproto.INetCopyMessage
		err := srv.GetNextMessage(&message)
		if err != nil {
			return err
		}

		switch message.(type) {

		case ncproto.FileChunk:
			chunk := message.(ncproto.FileChunk)
			if chunk.ConnectionID != conf.ConnectionID {
				fmt.Fprintf(os.Stderr, "loop: got file chunk from %s but expected it from someone else\n", conf.ConnectionID.String())
				continue
			}
			file, found := knownFiles[chunk.ID]
			if !found {
				return fmt.Errorf("unknown file for chunk %v", chunk)
			}

			file.ChunkQueue <- chunk

		case ncproto.File:
			file := message.(ncproto.File)

			if file.ConnectionID != conf.ConnectionID {
				fmt.Fprintf(os.Stderr, "loop: got file from %s but expected it from %s\n", file.ConnectionID.String(), conf.ConnectionID.String())
				continue
			}

			if !conf.Quiet {
				fmt.Printf("%s (%s)\n", filepath.Join(filepath.Join(file.RelativePath...), file.Name), file.PrettySize())
			}

			err = os.MkdirAll(filepath.Dir(file.FullFilePath(&conf)), 0775)
			if err != nil {
				fmt.Fprintf(os.Stderr, "loop: %v\n", err)
			}

			fd, err := os.OpenFile(file.FullFilePath(&conf), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0775)
			if err != nil {
				fmt.Fprintf(os.Stderr, "loop: %v\n", err)
			}

			file.FileDescriptor = fd
			file.ChunkQueue = make(chan ncproto.FileChunk)
			knownFiles[file.ID] = file

			fwg.Add(1)

			go func(iFile *ncproto.File, ifwg *sync.WaitGroup) {

				defer ifwg.Done()
				defer iFile.FileDescriptor.Close()

				for chunk := range iFile.ChunkQueue {

					n, err := iFile.FileDescriptor.Write(chunk.Data)
					if err != nil {
						fmt.Fprintf(os.Stderr, "loop: error writing chunk %d to file %s: %v\n", chunk.Seq, iFile.RelativeFilePath(&conf), err)
						return
					}

					if n != len(chunk.Data) {
						fmt.Fprintf(os.Stderr, "loop: expected to write %d bytes but wrote %d bytes\n", len(chunk.Data), n)
						return
					}
				}
			}(&file, &fwg)

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

			if completeMsg.ConnectionID != conf.ConnectionID {
				fmt.Fprintf(os.Stderr, "loop: got complete message from %s but expected it from %s\n", completeMsg.ConnectionID.String(), conf.ConnectionID.String())
				continue
			}

			file := knownFiles[completeMsg.ID]
			close(file.ChunkQueue)

		case ncproto.ConnectionClose:
			cc := message.(ncproto.ConnectionClose)

			if cc.ConnectionID != conf.ConnectionID {
				fmt.Fprintf(os.Stderr, "loop: got close message from %s but expected it from %s\n", cc.ConnectionID.String(), conf.ConnectionID.String())
				continue
			}
			fmt.Println("client says done. closing connection.")
			//srv.Connection.Close()
			break outer
			//os.Exit(0)

		case ncproto.Config:
			fmt.Fprintf(os.Stderr, "loop: initial MsgConfig already received.\n")
			continue
		}
	}

	fmt.Println("waiting for all files to be written")
	fwg.Wait()
	return nil
}

func init() {
	rootCmd.AddCommand(receiveCmd)

	receiveCmd.Flags().Uint16VarP(&conf.Port, "port", "p", 0, "set the port to listen to. If not set a random, available port is selected")
	receiveCmd.Flags().StringVarP(&conf.WorkingDirectory, "working-dir", "d", ".", "set the directory to output files to")
	receiveCmd.Flags().BoolVarP(&conf.Quiet, "quiet", "q", false, "don't print each received file nor transfer progress")

}
