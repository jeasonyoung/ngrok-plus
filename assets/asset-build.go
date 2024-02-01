package main

import (
	"fmt"
	"ngrok-plus/bindata"
	"os"
)

func main() {
	c := bindata.NewConfig()
	c.Package = "assets"
	c.Tags = ""
	c.Input = []bindata.InputConfig{
		{"../assets/server", true},
	}
	c.Output = "../ngrok/server/assets/assets.go"
	err := bindata.Translate(c)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "bindata: %v\n", err)
		os.Exit(1)
	}
}
