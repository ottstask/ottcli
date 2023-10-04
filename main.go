package main

import (
	"embed"

	"github.com/ottstask/ottcli/cmd"
)

//go:embed tmp_build/*
var content embed.FS

func main() {
	cmd.Execute(content)
}
