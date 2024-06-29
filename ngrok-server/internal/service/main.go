package service

import (
	"ngrok-common/conn"
	"time"
)

const (
	registryCacheSize uint64        = 1024 * 1024 //1MB
	connReadTimeout   time.Duration = 10 * time.Second
)

// GLOBALS
var (
	opts       *Options
	listerners map[string]*conn.Listener
)

// Main 主程序入口
func Main() {
	//parse options
	opts = parseArgs()
	//TODO:
}
