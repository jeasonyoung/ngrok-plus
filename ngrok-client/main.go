package main

import (
	_ "ngrok-client/internal/packed"

	"github.com/gogf/gf/v2/os/gctx"

	"ngrok-client/internal/cmd"
)

func main() {
	cmd.Main.Run(gctx.GetInitCtx())
}
