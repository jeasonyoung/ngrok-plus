package main

import (
	_ "ngrok-server/internal/packed"

	"github.com/gogf/gf/v2/os/gctx"

	"ngrok-server/internal/cmd"
)

func main() {
	cmd.Main.Run(gctx.GetInitCtx())
}
