package cmd

import (
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func setupWorkingDir(cmd *cobra.Command, args []string) {
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

	}
}
