package main

import (
	_ "ngrok-server/internal/packed"
	"ngrok-server/internal/service"
)

func main() {
	service.Main()
}
