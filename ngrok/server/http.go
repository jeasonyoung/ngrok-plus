package server

import (
	"crypto/tls"
	"fmt"
	vhost "github.com/inconshreveable/go-vhost"
	"ngrok-plus/ngrok/conn"
	"ngrok-plus/ngrok/log"
	"strings"
	"time"
)

const (
	NotAuthorized = `HTTP/1.0 401 Not Authorized
WWW-Authenticate: Basic realm="ngrok"
Content-Length: 23

Authorization required
`

	NotFound = `HTTP/1.0 404 Not Found
Content-Length: %d

Tunnel %s not found
`

	BadRequest = `HTTP/1.0 400 Bad Request
Content-Length: 12

Bad Request
`
)

// Listens for new http(s) connections from the public internet
func startHttpListener(addr string, tlsCfg *tls.Config) (listener *conn.Listener) {
	// bind/listen for incoming connections
	var err error
	if listener, err = conn.Listen(addr, "pub", tlsCfg); err != nil {
		panic(err)
	}

	proto := "http"
	if tlsCfg != nil {
		proto = "https"
	}

	log.Info("Listening for public %s connections on %v", proto, listener.Addr.String())
	go func() {
		for _conn := range listener.Conns {
			go httpHandler(_conn, proto)
		}
	}()

	return
}

// Handles a new http connection from the public internet
func httpHandler(c conn.Conn, proto string) {
	defer func(co conn.Conn) {
		_ = co.Close()
	}(c)
	defer func() {
		// recover from failures
		if r := recover(); r != nil {
			_ = c.Warn("httpHandler failed with error %v", r)
		}
	}()

	// Make sure we detect dead connections while we decide how to multiplex
	_ = c.SetDeadline(time.Now().Add(connReadTimeout))

	// multiplex by extracting the Host header, the vhost library
	vhostConn, err := vhost.HTTP(c)
	if err != nil {
		_ = c.Warn("Failed to read valid %s request: %v", proto, err)
		_, _ = c.Write([]byte(BadRequest))
		return
	}

	// read out the Host header and auth from the request
	host := strings.ToLower(vhostConn.Host())
	auth := vhostConn.Request.Header.Get("Authorization")

	// done reading mux data, free up the request memory
	vhostConn.Free()

	// We need to read from the vhost conn now since it mucked around reading the stream
	c = conn.Wrap(vhostConn, "pub")

	// multiplex to find the right backend host
	c.Debug("Found hostname %s in request", host)
	tunnel := tunnelRegistry.Get(fmt.Sprintf("%s://%s", proto, host))
	if tunnel == nil {
		c.Info("No tunnel found for hostname %s", host)
		_, _ = c.Write([]byte(fmt.Sprintf(NotFound, len(host)+18, host)))
		return
	}

	// If the client specified http auth and it doesn't match this request's auth
	// then fail the request with 401 Not Authorized and request the client reissue the
	// request with basic authdeny the request
	if tunnel.req.HttpAuth != "" && auth != tunnel.req.HttpAuth {
		c.Info("Authentication failed: %s", auth)
		_, _ = c.Write([]byte(NotAuthorized))
		return
	}

	// dead connections will now be handled by tunnel heartbeating and the client
	_ = c.SetDeadline(time.Time{})

	// let the tunnel handle the connection now
	tunnel.HandlePublicConnection(c)
}
