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
	"net"
	"time"

	"github.com/google/uuid"

	"github.com/bdoner/net-copy/shared"
	"github.com/spf13/cobra"
)

type sendConf struct {
	connectAddr string
	connectPort uint16
	inDir       string
}

var sconf sendConf

// sendCmd represents the send command
var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Set net-copy to send files.",
	/*Long: `A longer description that spans multiple lines and likely contains examples
	and usage of using your command. For example:

	Cobra is a CLI library for Go that empowers applications.
	This application is a tool to generate the needed files
	to quickly create a Cobra application.`,*/
	Run: func(cmd *cobra.Command, args []string) {

		m := collectFiles()
		conn := createConnection()

		enc := gob.NewEncoder(conn)
		enc.Encode(m)

		time.Sleep(time.Minute * 1)
	},
}

func collectFiles() shared.SetupMessage {
	fs := make([]shared.NCFile, 1)
	fs[0] = shared.NCFile{
		ID:       uuid.New(),
		FileSize: 1111,
		Name:     "testfile.bin",
	}
	m := shared.SetupMessage{
		NumFiles: 1,
		Files:    &fs,
	}

	return m
}

func createConnection() net.Conn {
	connAddr := fmt.Sprintf("%s:%d", sconf.connectAddr, sconf.connectPort)
	conn, err := net.Dial("tcp", connAddr)
	if err != nil {
		log.Fatalf("net-copy/send: could not establish connection to %s. %v\n", connAddr, err)
	}

	return conn
}

func init() {
	rootCmd.AddCommand(sendCmd)

	// Here you will define your flags and configuration settings.
	sendCmd.Flags().StringVarP(&sconf.connectAddr, "host", "a", "127.0.0.1", "Define which host to connect to")
	sendCmd.Flags().Uint16VarP(&sconf.connectPort, "port", "p", 0, "Receivers listen port to connect to.")
	sendCmd.Flags().StringVarP(&sconf.inDir, "in-dir", "d", ".", "The directory to copy files from")
	sendCmd.MarkFlagRequired("host")
	sendCmd.MarkFlagRequired("port")

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// sendCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// sendCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
