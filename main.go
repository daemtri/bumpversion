package main

import "github.com/duanqy/bumpversion/cmd"

var (
	Version string = "0.0.0"
)

func main() {
	cmd.Version = Version
	cmd.Execute()
}
