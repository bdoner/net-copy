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
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/bdoner/net-copy/ncproto"

	"github.com/spf13/cobra"
)

var conf ncproto.Config

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Set net-copy to send files",
	Long: `
	Connects to a host, given by -a, using the port given by -p, then collects
	a list of files to send. Once the connection is established net-copy will start
	sending all the files recursively found in the working-directory (-d).
	Once done the sender signals to the receiver it is done and the connection is closed.

	If -t is provided the lowest value between the sender and receiver is used.`,
	PreRun: setupConfig,
	Run: func(cmd *cobra.Command, args []string) {

		conn := createConnection()
		defer conn.Close()

		enc := gob.NewEncoder(conn)
		enc.Encode(ncproto.MsgConfig)
		enc.Encode(conf)

		readBuffer := make([]byte, conf.ReadBufferSize)
		files := make([]ncproto.File, 0)
		collectFiles(conf.WorkingDirectory, &files)
		for _, file := range files {
			fmt.Printf("%s\n", file.Name)

			fp, err := os.Open(file.FullPath(&conf))
			if err != nil {
				fmt.Fprintf(os.Stderr, "error opening file %s", filepath.Join(file.RelativePath, file.Name))
			}

			enc.Encode(ncproto.MsgFile)
			enc.Encode(file)
			sentChunks := 0
			for {
				n, err := fp.Read(readBuffer)
				if n == 0 && err == io.EOF {
					break
				}
				if err != nil {
					fmt.Fprintf(os.Stderr, "error reading file %s", filepath.Join(file.RelativePath, file.Name))
					break
				}

				fchunk := ncproto.FileChunk{
					ID:           file.ID,
					ConnectionID: conf.ConnectionID,
					Data:         readBuffer[:n],
				}
				sentChunks++
				//fmt.Printf("sending chunk %d\n", sentChunks)
				enc.Encode(fchunk)
			}

		}

		enc.Encode(ncproto.MsgClose)
		enc.Encode(ncproto.ConnectionClose{ConnectionID: conf.ConnectionID})

		//time.Sleep(time.Minute * 1)
		//files := collectFiles()
	},
}

func collectFiles(dir string, files *[]ncproto.File) {
	fs, err := ioutil.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading %s. %v\n", dir, err)
	}

	for _, v := range fs {
		if v.IsDir() {
			collectFiles(filepath.Join(dir, v.Name()), files)
		} else {
			rel, err := filepath.Rel(conf.WorkingDirectory, dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				continue
			}
			nf := ncproto.File{
				ID:           uuid.New(),
				ConnectionID: conf.ConnectionID,
				FileSize:     v.Size(),
				Name:         v.Name(),
				RelativePath: rel,
			}
			*files = append(*files, nf)
		}
	}

}

func createConnection() net.Conn {
	connAddr := fmt.Sprintf("%s:%d", conf.Hostname, conf.Port)
	conn, err := net.Dial("tcp", connAddr)
	if err != nil {
		log.Fatalf("net-copy/send: could not establish connection to %s. %v\n", connAddr, err)
	}

	return conn
}

func init() {
	rootCmd.AddCommand(sendCmd)

	// Here you will define your flags and configuration settings.
	sendCmd.Flags().StringVarP(&conf.Hostname, "host", "a", "", "define which host to connect to")
	sendCmd.Flags().Uint16VarP(&conf.Port, "port", "p", 0, "the port to connect to")
	sendCmd.Flags().StringVarP(&conf.WorkingDirectory, "working-dir", "d", ".", "the directory to copy files from")
	sendCmd.Flags().Uint16VarP(&conf.Threads, "threads", "t", 1, "define how many concurrent transfers to run")
	sendCmd.MarkFlagRequired("host")
	sendCmd.MarkFlagRequired("port")

	conf.ConnectionID = uuid.New()
	conf.ReadBufferSize = 32 * 1024

}
