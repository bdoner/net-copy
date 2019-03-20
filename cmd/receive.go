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
	"os"

	"github.com/bdoner/net-copy/ncproto"
	"github.com/spf13/cobra"
)

var rconf ncproto.Config

// receiveCmd represents the receive command
var receiveCmd = &cobra.Command{
	Use:   "receive",
	Short: "Set net-copy to receive files",
	/*Long: `A longer description that spans multiple lines and likely contains examples
	and usage of using your command. For example:

	Cobra is a CLI library for Go that empowers applications.
	This application is a tool to generate the needed files
	to quickly create a Cobra application.`,*/
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
			fmt.Println("client says done. closing connection.")
			(*conn).Close()
			os.Exit(0)

		case ncproto.MsgFile:
			var f ncproto.File
			err := dec.Decode(&f)
			if err != nil {
				fmt.Fprintf(os.Stderr, "net-copy/receive: error decoding file message. %v\n", err)
			}

			fmt.Printf("Got file %s of size %s\n", f.Name, f.PrettySize())

		case ncproto.MsgConfig:
			log.Fatal("net-copy/receive: initial MsgConfig already received.\n")
		}
	}
}

func getConnection() net.Conn {
	l, err := net.Listen("tcp4", fmt.Sprintf(":%d", conf.Port))
	if err != nil {
		log.Fatalf("netcopy/receive: could not listen on port %d\n", conf.Port)
	}

	fmt.Printf("Listening on %s\n", l.Addr().String())
	conn, err := l.Accept()
	if err != nil {
		log.Fatalf("netcopy/receive: could not accept initial connection\n")
	}

	return conn
}

func init() {
	rootCmd.AddCommand(receiveCmd)

	receiveCmd.Flags().Uint16VarP(&conf.Port, "port", "p", 0, "Set the port to listen to. If not set a random, available port is selected")
	receiveCmd.Flags().StringVarP(&conf.WorkingDirectory, "working-dir", "d", ".", "Set the directory to output files to.")

	if conf.WorkingDirectory == "." {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatalf("net-copy/send: could not get cwd. Please specify a working directory manually: %v\n", err)
		}
		conf.WorkingDirectory = wd
	}
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// receiveCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// receiveCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
