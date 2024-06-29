package service

import (
	"crypto/tls"
	"math/rand"
	"ngrok-common/conn"
	"ngrok-common/log"
	"ngrok-common/msg"
	"ngrok-common/utility"
	"runtime/debug"
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

func NewProxy(pxyConn conn.Conn, regPxy *msg.RegProxy) {
	// fail gracefully if the proxy connection fails to register
	defer func() {
		if err := recover(); err != nil {
			_ = pxyConn.Warn("Failed with error: %v", err)
		}
	}()

	// set logging prefix
	pxyConn.SetType("pxy")

	// look up the control connection for this proxy
	pxyConn.Info("Registering new proxy for %s", regPxy.ClientId)
	ctl := ControlRegistry().Get(regPxy.ClientId)
	if ctl == nil {
		panic("No client found for identifier: " + regPxy.ClientId)
	}

	ctl.RegisterProxy(pxyConn)
}

// Listen for incoming control and proxy connections we listen for incoming control and proxy connections on the same port
// for ease of deployment.The hope is that by running on port 443,using TLS and running all connections over the same port
// we can bust through restrictive firewalls.
func tunnelListener(addr string, tlsConfig *tls.Config) {
	// listen for incoming connections
	listener, err := conn.Listen(addr, "tun", tlsConfig)
	if err != nil {
		panic(err)
	}
	log.Info("Listening for control and proxy connections on %s", listener.Addr.String())
	for c := range listener.Connections {
		go func(tunnelConn conn.Conn) {
			// don's crash on panics
			defer func() {
				if r := recover(); r != nil {
					tunnelConn.Info("tunnelListener failed with error %v: %s", r, debug.Stack())
				}
			}()

			_ = tunnelConn.SetDeadline(time.Now().Add(connReadTimeout))
			var rawMsg msg.Message
			if rawMsg, err = msg.ReadMsg(tunnelConn); err != nil {
				_ = tunnelConn.Warn("Failed to read message: %v", err)
				_ = tunnelConn.Close()
				return
			}

			// don't time out after the initital read, tunnel hearting will kill dead connections
			_ = tunnelConn.SetWriteDeadline(time.Time{})

			switch m := rawMsg.(type) {
			case *msg.Auth:
				NewControl(tunnelConn, m)
			case *msg.RegProxy:
				NewProxy(tunnelConn, m)

			default:
				_ = tunnelConn.Close()
			}
		}(c)
	}
}

// Main 主程序入口
func Main() {
	//parse options
	opts = parseArgs()

	//init logging
	log.LogTo(opts.logTo, opts.logLevel)

	// seed random number generator
	seed, err := utility.RandomSeed()
	if err != nil {
		panic(err)
	}
	rand.Seed(seed)

	// start listeners
	listerners = make(map[string]*conn.Listener)

	// load tls configuration
	var tlsConfig *tls.Config
	if tlsConfig, err = LoadTLSConfig(opts.tlsCrt, opts.tlsKey); err != nil {
		panic(err)
	}

	// listen for http
	if opts.httpAddr != "" {
		listerners["http"] = startHttpListener(opts.httpAddr, nil)
	}

	// listen for https
	if opts.httpsAddr != "" {
		listerners["https"] = startHttpListener(opts.httpsAddr, tlsConfig)
	}
	// ngrok-plus clients
	tunnelListener(opts.tunnelAddr, tlsConfig)
}
