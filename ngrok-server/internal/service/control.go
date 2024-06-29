package service

import (
	"fmt"
	"io"
	"ngrok-common/conn"
	"ngrok-common/msg"
	"ngrok-common/utility"
	"ngrok-common/version"
	"runtime/debug"
	"strings"
	"time"
)

const (
	pingTimeoutInterval = 30 * time.Second
	connReapInterval    = 10 * time.Second
	controlWriteTimeout = 10 * time.Second
	proxyStaleDuration  = 60 * time.Second
	proxyMaxPoolSize    = 10
)

type Control struct {
	// auth message
	auth *msg.Auth
	// actual connection
	conn conn.Conn
	// put a message in this channel to send it over conn to the client.
	out chan msg.Message
	// read from this channel to get the next message sent to us over conn by the client.
	in chan msg.Message
	// the last time we received a ping from the client - for heartbeats
	lastPing time.Time
	// all tunnels this control connection handles
	tunnels []*Tunnel
	// proxy connetions
	proxies chan conn.Conn
	// identifier
	id string
	// synchronizer for controlled shutdown of writer()
	writerShutdown *utility.Shutdown
	// synchronizer for controlled shutdown of reader()
	readerShutdown *utility.Shutdown
	// synchronizer for controlled shutdown of manager()
	managerShutdown *utility.Shutdown
	// synchronizer for controlled shutdown of entire Control
	shutdown *utility.Shutdown
}

// NewControl 新建控制器
func NewControl(ctlConn conn.Conn, authMsg *msg.Auth) {
	var err error
	// create the object
	c := &Control{
		auth:            authMsg,
		conn:            ctlConn,
		out:             make(chan msg.Message),
		in:              make(chan msg.Message),
		proxies:         make(chan conn.Conn, 10),
		lastPing:        time.Now(),
		writerShutdown:  utility.NewShutdown(),
		readerShutdown:  utility.NewShutdown(),
		managerShutdown: utility.NewShutdown(),
		shutdown:        utility.NewShutdown(),
	}

	failAuth := func(e error) {
		_ = msg.WriteMsg(ctlConn, &msg.AuthRes{Error: e.Error()})
		_ = ctlConn.Close()
	}

	// register the clientId
	c.id = authMsg.ClientId
	if c.id == "" {
		// it's a new session, assign an ID
		if c.id, err = utility.SecureRandId(16); err != nil {
			failAuth(err)
			return
		}
	}

	// set logging prefix
	ctlConn.SetType("ctl")
	ctlConn.AddLogPrefix(c.id)

	//检查版本
	if authMsg.Version != version.Proto {
		failAuth(fmt.Errorf("incompatible versions. Server %s,client %s. Download a new version at https://ngrok-plus.com", version.MajorMinor(), authMsg.Version))
		return
	}

	//register the control
	if replaced := ControlRegistry().Add(c.id, c); replaced != nil {
		replaced.shutdown.WaitComplete()
	}

	// start the writer first so that the following messages get sent
	go c.writer()

	//Respond to authentication
	c.out <- &msg.AuthRes{
		Version:   version.Proto,
		MmVersion: version.MajorMinor(),
		ClientId:  c.id,
	}

	// As a performance optimization, ask for a proxy connection up front
	c.out <- &msg.ReqProxy{}

	//manage the connection
	go c.manager()
	go c.reader()
	go c.stopper()
}

func (c *Control) writer() {
	defer func() {
		if err := recover(); err != nil {
			c.conn.Info("Control::writer failed with error %v:%s", err, debug.Stack())
		}
	}()

	//kill everything if the writer() stops
	defer c.shutdown.Begin()

	//notify that we've flushed all message
	defer c.writerShutdown.Complete()

	//write message to the control channel
	for m := range c.out {
		_ = c.conn.SetWriteDeadline(time.Now().Add(controlWriteTimeout))
		if err := msg.WriteMsg(c.conn, m); err != nil {
			panic(err)
		}
	}
}

func (c *Control) manager() {
	// don't crash on panics
	defer func() {
		if err := recover(); err != nil {
			c.conn.Info("Control::manager failed with error %v:%s", err, debug.Stack())
		}
	}()

	// kill everything if the control manager stops
	defer c.shutdown.Begin()

	// notify that manager() has shutdown
	defer c.managerShutdown.Complete()

	//reaping timer for detcting heartbeat failure
	reap := time.NewTimer(connReapInterval)
	defer reap.Stop()

	for {
		select {
		case <-reap.C:
			if time.Since(c.lastPing) > pingTimeoutInterval {
				c.conn.Info("Lost heartbeat")
				c.shutdown.Begin()
			}
		case mRaw, ok := <-c.in:
			// c.in closes to indicate shutdown
			if !ok {
				return
			}
			switch m := mRaw.(type) {
			case *msg.ReqTunnel:
				c.registerTunnel(m)

			case *msg.Ping:
				c.lastPing = time.Now()
				c.out <- &msg.Pong{}
			}
		}
	}
}

func (c *Control) reader() {
	defer func() {
		if err := recover(); err != nil {
			_ = c.conn.Warn("Control::reader failed with error %v:%s", err, debug.Stack())
		}
	}()

	// kill everything if the reader stops
	defer c.shutdown.Begin()

	//notify that we're done
	defer c.readerShutdown.Complete()

	//reader message from the control channel
	for {
		if data, err := msg.ReadMsg(c.conn); err != nil {
			if err == io.EOF {
				c.conn.Info("EOF")
				return
			} else {
				panic(err)
			}
		} else {
			// this can also panic during shutdown
			c.in <- data
		}
	}
}

func (c *Control) stopper() {
	defer func() {
		if err := recover(); err != nil {
			_ = c.conn.Error("Failed to shutdown control: %v", err)
		}
	}()

	//wait until we're instructed shutdown
	c.shutdown.WaitBegin()

	// remove our self from the control registry
	_ = ControlRegistry().Del(c.id)

	// shutdown manager() so that we have no more work to do
	close(c.in)
	c.managerShutdown.WaitComplete()

	// shutdown writer()
	close(c.out)
	c.writerShutdown.WaitComplete()

	// close connection fully
	_ = c.conn.Close()

	// shutdown all the tunnels
	for _, t := range c.tunnels {
		t.Shutdown()
	}

	// shutdown all the proxy connections
	close(c.proxies)
	for p := range c.proxies {
		_ = p.Close()
	}

	c.shutdown.Complete()
	c.conn.Info("Shutdown complete.")
}

// register a new tunnel on this control connection
func (c *Control) registerTunnel(rawTunnelReq *msg.ReqTunnel) {
	for _, proto := range strings.Split(rawTunnelReq.Protocol, "+") {
		tunnelReq := *rawTunnelReq
		tunnelReq.Protocol = proto

		c.conn.Debug("Registering new tunnel[proto:%s]", proto)
		t, err := NewTunnel(&tunnelReq, c)
		if err != nil {
			c.out <- &msg.NewTunnel{Error: err.Error()}
			if len(c.tunnels) == 0 {
				c.shutdown.Begin()
			}
			// we're done
			return
		}

		// add it to the list of tunnels
		c.tunnels = append(c.tunnels, t)

		//acknowledge success
		c.out <- &msg.NewTunnel{
			Url:      t.url,
			Protocol: proto,
			ReqId:    rawTunnelReq.ReqId,
		}

		rawTunnelReq.Hostname = strings.Replace(t.url, proto+"://", "", 1)
	}
}

// RegisterProxy 注册代理
func (c *Control) RegisterProxy(conn conn.Conn) {
	conn.AddLogPrefix(c.id)

	_ = conn.SetDeadline(time.Now().Add(proxyStaleDuration))
	select {
	case c.proxies <- conn:
		conn.Info("Registered")
	default:
		conn.Info("Proxies buffer is full, discarding.")
		_ = conn.Close()
	}
}

// GetProxy Remove a proxy connection from the pool and return it
// If not proxy connections are in the pool,request one and wait until it is available
// Returns an error if we couldn't get a proxy because it took too long or the tunnel is closing
func (c *Control) GetProxy() (proxyConn conn.Conn, err error) {
	var ok bool

	// get a proxy connection from the pool
	select {
	case proxyConn, ok = <-c.proxies:
		if !ok {
			err = fmt.Errorf("no proxy connections available, control is closing")
			return
		}
	default:
		// no proxy available in the pool,ask for one over the control channel
		c.conn.Debug("No proxy in pool,requesting proxy from control...")
		if err = utility.PanicToError(func() { c.out <- &msg.ReqProxy{} }); err != nil {
			return
		}

		select {
		case proxyConn, ok = <-c.proxies:
			if !ok {
				err = fmt.Errorf("no proxy connections available,control is closing")
				return
			}

		case <-time.After(pingTimeoutInterval):
			err = fmt.Errorf("timeout trying to get proxy connection")
			return
		}
	}
	return
}

// Replaced Called when this control is replaced by another control
// this can happen if the network drops out and the client reconnects
// before the old tunnel has lost its heartbeat
func (c *Control) Replaced(replacement *Control) {
	c.conn.Info("Replaced by control: %s", replacement.conn.Id())

	// set the control id to empty string so that when stopper()
	// calls registry.Del it won't delete the replacement
	c.id = ""

	// tell the old one to shut down
	c.shutdown.Begin()
}
