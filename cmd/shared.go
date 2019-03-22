package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func setupConfig(cmd *cobra.Command, args []string) {
	if conf.WorkingDirectory == "." {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatalf("net-copy/send: could not get cwd. Please specify a working directory manually: %v\n", err)
		}
		conf.WorkingDirectory = wd

	} else {
		abs, err := filepath.Abs(conf.WorkingDirectory)
		if err != nil {
			log.Fatalf("net-copy/send: could not get absolute path: %v\n", err)
		}

		conf.WorkingDirectory = abs

		_, err = os.Open(conf.WorkingDirectory)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Printf("Output directory does not exists. creating %s\n", conf.WorkingDirectory)
				err := os.MkdirAll(conf.WorkingDirectory, 0775)
				if err != nil {
					log.Fatalf("could not create output directory: %v\n", err)
				}
			} else {
				log.Fatalf("could not open output directory: %v\n", err)
			}
		}

		wdFiles, err := ioutil.ReadDir(conf.WorkingDirectory)
		if err != nil {
			log.Fatalf("could not open output directory: %v\n", err)
		}

		if 0 < len(wdFiles) {
			log.Fatalf("can only output into an empty directory\n%s is not empty\n", conf.WorkingDirectory)
		}

	}
}
