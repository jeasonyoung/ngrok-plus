package service

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"net"
	"ngrok-common/conn"
	"ngrok-common/log"
	"ngrok-common/msg"
	"ngrok-common/utility"
	"ngrok-server/internal/consts"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var defaultPortMap = map[string]int{
	"http":  80,
	"https": 443,
	"smtp":  25,
}

// Tunnel A control connection,metadata and proxy connections which route public traffic to a firewalled endpoint.
type Tunnel struct {
	// request that opened the tunnel
	req *msg.ReqTunnel
	// time when the tunnel was opened
	start time.Time
	// public url
	url string
	// tcp listener
	listener *net.TCPListener
	// control connection
	ctl *Control
	// logger
	log.Logger
	//closing
	closing int32
}

// Common functionality for registering virtually hosted protocols
func registerVhost(t *Tunnel, protocol string, servingPort int) (err error) {
	vhost := os.Getenv(consts.VHost)
	if vhost == "" {
		vhost = fmt.Sprintf("%s:%d", opts.domain, servingPort)
	}
	// Canonicalize virtual host by removing default port (e.g :80 on HTTP)
	defaultPort, ok := defaultPortMap[protocol]
	if !ok {
		return fmt.Errorf("couldn't find default port for protocol %s", protocol)
	}

	defaultPortSuffix := fmt.Sprintf(":%d", defaultPort)
	if strings.HasSuffix(vhost, defaultPortSuffix) {
		vhost = vhost[0 : len(vhost)-len(defaultPortSuffix)]
	}

	// canonicalize by always using lower-case
	vhost = strings.ToLower(vhost)

	tunnelRegistry := TunnelRegistry()
	// Register for spcific hostname
	hostname := strings.ToLower(strings.TrimSpace(t.req.Hostname))
	if hostname != "" {
		t.url = fmt.Sprintf("%s://%s", protocol, hostname)
		return tunnelRegistry.Register(t.url, t)
	}

	// Register for specific subdomain
	subdomain := strings.ToLower(strings.TrimSpace(t.req.Subdomain))
	if subdomain != "" {
		t.url = fmt.Sprintf("%s://%s.%s", protocol, subdomain, vhost)
		return tunnelRegistry.Register(t.url, t)
	}

	// Register for random URL
	t.url, err = tunnelRegistry.RegisterRepeat(func() string {
		return fmt.Sprintf("%s://%x.%s", protocol, rand.Int31(), vhost)
	}, t)

	return
}

// NewTunnel Create a new tunnel from a registeration message receivd on a control channel
func NewTunnel(m *msg.ReqTunnel, ctl *Control) (t *Tunnel, err error) {
	t = &Tunnel{
		req:    m,
		start:  time.Now(),
		ctl:    ctl,
		Logger: log.NewPrefixLogger(),
	}

	proto := t.req.Protocol
	switch proto {
	case "tcp":
		bindTcp := func(port int) error {
			if t.listener, err = net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: port}); err != nil {
				err = t.ctl.conn.Error("Error binding TCP listener: %v", err)
				return err
			}

			//create the url
			addr := t.listener.Addr().(*net.TCPAddr)
			t.url = fmt.Sprintf("tcp://%s:%d", opts.domain, addr.Port)

			// register it
			if err = TunnelRegistry().RegisterAndCache(t.url, t); err != nil {
				// This should never be possible because the OS will only assign available ports to us.
				_ = t.listener.Close()

				err = fmt.Errorf("TCP listener bound,but failed to register %s", t.url)
				return err
			}

			go t.listenTcp(t.listener)
			return nil
		}

		// use the custom remote port you asked for
		if t.req.RemotePort != 0 {
			_ = bindTcp(int(t.req.RemotePort))
			return
		}

		// try to return to you the same port you had before
		cacheUrl := TunnelRegistry().GetCachedRegistration(t)
		if cacheUrl != "" {
			parts := strings.Split(cacheUrl, ":")
			portPart := parts[len(parts)-1]
			var port int
			if port, err = strconv.Atoi(portPart); err != nil {
				_ = t.ctl.conn.Error("Failed to parse cached url port as integer: %s", portPart)
			} else {
				// we have a valid,cache port,let's try to bind with it
				if bindTcp(port) != nil {
					_ = t.ctl.conn.Warn("Failed to get custom port %d:%v,trying a random one", port, err)
				} else {
					// success,we're done
					return
				}
			}
		}
		// bind for tcp connections
		_ = bindTcp(0)
		return

	case "http", "https":
		l, ok := listerners[proto]
		if !ok {
			err = fmt.Errorf("not listening for %s connections", proto)
			return
		}
		if err = registerVhost(t, proto, l.Addr.(*net.TCPAddr).Port); err != nil {
			return
		}
	default:
		err = fmt.Errorf("protocol %s is not supported", proto)
		return
	}

	// pre-encode the http basic auth for fast comparisons later
	if m.HttpAuth != "" {
		m.HttpAuth = "Basic " + base64.StdEncoding.EncodeToString([]byte(m.HttpAuth))
	}

	t.AddLogPrefix(t.Id())
	t.Info("Registered new tunnel on: %s", t.ctl.conn.Id())

	Metrics().OpenTunnel(t)
	return
}

func (t *Tunnel) Shutdown() {
	t.Info("Shutting down")

	//mark that we're shutting down
	atomic.StoreInt32(&t.closing, 1)

	// if we have a public listener (this is a raw TCP tunnel), shut it down
	if t.listener != nil {
		_ = t.listener.Close()
	}

	// remove ourselves from the tunnel registry
	TunnelRegistry().Del(t.url)

	// let the control connection know we're shutting down
	// currently, only the control connection shuts down tunnels
	// so it doesn't need to know about it
	// t.ctl.stoptunnel <- t
	Metrics().CloseTunnel(t)
}

// Id 通道ID
func (t *Tunnel) Id() string {
	return t.url
}

// listens for new public tcp connections from the internet
func (t *Tunnel) listenTcp(listener *net.TCPListener) {
	for {
		defer func() {
			if err := recover(); err != nil {
				_ = log.Warn("listen Tcp failed with err %v", err)
			}
		}()

		// accept public connections
		tcpConn, err := listener.AcceptTCP()
		if err != nil {
			// not an error,we're shutting down this tunnel
			if atomic.LoadInt32(&t.closing) == 1 {
				return
			}

			_ = t.Error("Fail to accept new TCP connection: %v", err)
			continue
		}

		_conn := conn.Wrap(tcpConn, "pub")
		_conn.AddLogPrefix(t.Id())
		_conn.Info("New connection from %v", _conn.RemoteAddr())

		go t.HandlePublicConnection(_conn)
	}
}

func (t *Tunnel) HandlePublicConnection(publicConn conn.Conn) {
	defer func(c conn.Conn) {
		_ = c.Close()
	}(publicConn)
	defer func() {
		if err := recover(); err != nil {
			_ = publicConn.Warn("HandlePublicConnection failed with error %v", err)
		}
	}()

	startTime := time.Now()
	Metrics().OpenConnection(t, publicConn)

	var proxyConn conn.Conn
	var err error
	for i := 0; i < (2 * proxyMaxPoolSize); i++ {
		// get a proxy connection
		if proxyConn, err = t.ctl.GetProxy(); err != nil {
			_ = t.Warn("Failed to get proxy connection: %v", err)
			return
		}
		defer func(p conn.Conn) {
			_ = p.Close()
		}(proxyConn)
		t.Info("Got proxy connection %s", proxyConn.Id())
		proxyConn.AddLogPrefix(t.Id())

		// tell the client we're going to start using this proxy connection
		startProxyMsg := &msg.StartProxy{
			Url:        t.url,
			ClientAddr: publicConn.RemoteAddr().String(),
		}

		if err = msg.WriteMsg(proxyConn, startProxyMsg); err != nil {
			_ = proxyConn.Warn("Failed to write StartProxyMessage: %v, attempt: %d", err, i)
			_ = proxyConn.Close()
		} else {
			// success
			break
		}
	}

	if err != nil {
		// give up
		_ = publicConn.Error("Too many failures starting proxy connection")
		return
	}

	// To reduce latency handing tunnel connections, we employ the following curde heuristic:
	// Whenever we take a proxy connection from the pool, replace it with a new one
	_ = utility.PanicToError(func() { t.ctl.out <- &msg.ReqProxy{} })

	// no timeouts while connections are joined
	_ = proxyConn.SetDeadline(time.Time{})

	// join the public and proxy connections
	bytesIn, bytesOut := conn.Join(publicConn, proxyConn)
	Metrics().CloseConnection(t, publicConn, startTime, bytesIn, bytesOut)
}

// GetUrl 获取URL
func (t *Tunnel) GetUrl() string {
	return t.url
}

// GetStart 获取开始时间
func (t *Tunnel) GetStart() time.Time {
	return t.start
}

// GetCtlIPAddr 获取控制IP地址
func (t *Tunnel) GetCtlIPAddr() string {
	return t.ctl.conn.RemoteAddr().(*net.TCPAddr).IP.String()
}

// GetCtlId 获取控制ID
func (t *Tunnel) GetCtlId() string {
	return t.ctl.id
}

// GetCtlAuthUser 获取控制认证用户
func (t *Tunnel) GetCtlAuthUser() string {
	return t.ctl.auth.User
}

// GetCtlAuthVersion 获取控制认证版本
func (t *Tunnel) GetCtlAuthVersion() string {
	return t.ctl.auth.MmVersion
}

// GetCtlAuthOS 获取操作系统
func (t *Tunnel) GetCtlAuthOS() string {
	return t.ctl.auth.OS
}

// GetReqProcotol 获取请求协议
func (t *Tunnel) GetReqProcotol() string {
	return t.req.Protocol
}

// GetReqHttpAuth 获取HTTP请求认证
func (t *Tunnel) GetReqHttpAuth() string {
	return t.req.HttpAuth
}

// GetReqSubdomain 获取请求子域名
func (t *Tunnel) GetReqSubdomain() string {
	return t.req.Subdomain
}
