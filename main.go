package main

import "github.com/ryomappi/kaiten/cmd"

var version = "dev"

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
