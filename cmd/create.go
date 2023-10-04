package cmd

import (
	"embed"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const tmpDir = "tmp_build/"

var (
	profile     string = ""
	goModPrefix string = ""

	serverNames = map[string]string{
		"scaffold":  "scaffold",
		"websocket": "websocket_server",
	}
)

func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.Flags().StringVarP(&profile, "profile", "p", "scaffold", "Profile name, choices: scaffold | websocket")
	createCmd.Flags().StringVarP(&goModPrefix, "mod", "m", "gkitserver", "modle name prefix in go.mod")
}

var createCmd = &cobra.Command{
	Use:     "create",
	Example: "gkit create demo",
	Short:   "Create scaffold project for gkit",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		serverName := args[0]
		if _, err := os.Stat(serverName); !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "%s directory exists", serverName)
			os.Exit(1)
		}
		p := tmpDir + profile
		replace := map[string]string{serverNames[profile]: serverName}
		if err := copyDir(content, p, serverName, replace); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		modContent := fmt.Sprintf("module %s/%s\n", goModPrefix, serverName)
		if err := ioutil.WriteFile(serverName+"/go.mod", []byte(modContent), 0755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("server %s created\n", serverName)
	},
}

func copyDir(content embed.FS, sourceDir, targetDir string, replace map[string]string) error {
	dirs, err := content.ReadDir(sourceDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}
	for _, d := range dirs {
		if d.IsDir() {
			p1 := sourceDir + "/" + d.Name()
			p2 := targetDir + "/" + d.Name()
			if err := copyDir(content, p1, p2, replace); err != nil {
				return err
			}
		} else if d.Type().IsRegular() {
			p := sourceDir + "/" + d.Name()
			bs, err := content.ReadFile(p)
			if err != nil {
				return err
			}
			p = targetDir + "/" + d.Name()
			c := string(bs)
			for k, v := range replace {
				c = strings.ReplaceAll(c, k, v)
			}
			if err := ioutil.WriteFile(p, []byte(c), 0755); err != nil {
				return err
			}
		}
	}
	return nil
}
