package cmd

import (
	"embed"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var content embed.FS

var rootCmd = &cobra.Command{
	Use:   "gkit",
	Short: "gkit is a command tool for gkit framework",
	Long: `gkit is a command tool for gkit framework
            Complete documentation is available at https://git.garena.com/shopee/sz-devops/OPD/framework/go-service`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
		os.Exit(1)
	},
}

func Execute(c embed.FS) {
	content = c
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
