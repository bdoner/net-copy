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
	"container/list"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/bdoner/net-copy/ncproto/ncclient"

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
	Once done the sender signals to the receiver it is done and the connection is closed.`,
	PreRun: setupWorkingDir,
	RunE: func(cmd *cobra.Command, args []string) error {

		cln, err := ncclient.Connect(conf.Hostname, conf.Port)
		if err != nil {
			return err
		}

		defer cln.Connection.Close()

		cln.SendMessage(conf)

		files := list.New()
		collectFiles(conf.WorkingDirectory, files)

		fmt.Printf("found %d files to transfer\n", files.Len())
		var wg sync.WaitGroup
	outer:
		for {

			for i := uint16(0); i < conf.Threads; i++ {
				wg.Add(1)

				e := files.Front()
				if e == nil {
					wg.Done()
					break outer
				}

				file := e.Value.(ncproto.File)
				files.Remove(e)

				go cln.SendFile(&file, &wg, &conf)
			}
			wg.Wait()
		}

		fmt.Println("waiting for last sends to complete..")
		wg.Wait()

		fmt.Println("all files sent. closing connection")
		cln.SendMessage(ncproto.ConnectionClose{
			ConnectionID: conf.ConnectionID,
		})

		return nil
	},
}

func collectFiles(dir string, files *list.List) {
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
				RelativePath: filepath.SplitList(rel),
			}

			files.PushBack(nf)
		}
	}

}

func init() {
	rootCmd.AddCommand(sendCmd)

	sendCmd.Flags().StringVarP(&conf.Hostname, "host", "a", "", "define which host to connect to")
	sendCmd.Flags().Uint16VarP(&conf.Port, "port", "p", 0, "the port to connect to")
	sendCmd.Flags().StringVarP(&conf.WorkingDirectory, "working-dir", "d", ".", "the directory to copy files from")
	sendCmd.Flags().Uint16VarP(&conf.Threads, "threads", "t", 1, "define how many concurrent transfers to run")
	sendCmd.Flags().BoolVarP(&conf.Quiet, "quiet", "q", false, "don't print each sent file nor transfer progress")
	sendCmd.MarkFlagRequired("host")
	sendCmd.MarkFlagRequired("port")

	conf.ConnectionID = uuid.New()
	conf.ReadBufferSize = 128 * 1024

}
