package conn

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	vhost "github.com/inconshreveable/go-vhost"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"ngrok-common/log"
	"sync"
)

type loggedConn struct {
	tcp *net.TCPConn
	net.Conn
	log.Logger
	id  int32
	typ string
}

func wrapConn(rawConn net.Conn, typ string) Conn {
	switch c := rawConn.(type) {
	case *vhost.HTTPConn:
		wrapped := c.Conn.(*loggedConn)
		return &loggedConn{wrapped.tcp, rawConn, wrapped.Logger, wrapped.id, wrapped.typ}
	case *loggedConn:
		return c
	case *net.TCPConn:
		wrapped := &loggedConn{c, rawConn, log.NewPrefixLogger(), rand.Int31(), typ}
		wrapped.AddLogPrefix(wrapped.Id())
		return wrapped
	}
	return nil
}

// Wrap 包装链接处理
func Wrap(rawConn net.Conn, typ string) Conn {
	return wrapConn(rawConn, typ)
}

// Listen 开启监听
func Listen(addr, typ string, tlsCfg *tls.Config) (l *Listener, err error) {
	// listen for incoming connections
	var listener net.Listener
	if listener, err = net.Listen("tcp", addr); err != nil {
		return
	}
	l = &Listener{
		Addr:        listener.Addr(),
		Connections: make(chan Conn),
	}
	go func() {
		for {
			rawConn, _err := listener.Accept()
			if _err != nil {
				_ = log.Error("Failed to accept new TCP connection of type %s:%v", typ, _err)
				continue
			}
			c := wrapConn(rawConn, typ).(*loggedConn)
			if tlsCfg != nil {
				c.Conn = tls.Server(c.Conn, tlsCfg)
			}
			c.Info("New connection from %v", c.RemoteAddr())
			l.Connections <- c
		}
	}()
	return
}

// Dial 拨号链接处理
func Dial(addr, typ string, tlsCfg *tls.Config) (s Conn, err error) {
	var rawConn net.Conn
	if rawConn, err = net.Dial("tcp", addr); err != nil {
		return
	}
	s = wrapConn(rawConn, typ)
	s.Debug("New connection to: %v", rawConn.RemoteAddr())
	if tlsCfg != nil {
		s.StartTLS(tlsCfg)
	}
	return
}

// DialHttpProxy 代理拨号链接处理
func DialHttpProxy(proxyUrl, addr, typ string, tlsCfg *tls.Config) (s Conn, err error) {
	// parse the proxy address
	var parsedUrl *url.URL
	if parsedUrl, err = url.Parse(proxyUrl); err != nil {
		return
	}
	var proxyAuth string
	if parsedUrl.User != nil {
		authToken := base64.StdEncoding.EncodeToString([]byte(parsedUrl.User.String()))
		proxyAuth = fmt.Sprintf("Basic %s", authToken)
	}
	var proxyTlsConfig *tls.Config
	switch parsedUrl.Scheme {
	case "http":
		proxyTlsConfig = nil
	case "https":
		proxyTlsConfig = new(tls.Config)
	default:
		err = fmt.Errorf("proxy URL scheme must be http or https, got: %s", parsedUrl.Scheme)
		return
	}
	// dial the proxy
	if s, err = Dial(parsedUrl.Host, typ, proxyTlsConfig); err != nil {
		return
	}
	// send an HTTP proxy CONNECT message
	var req *http.Request
	if req, err = http.NewRequest("CONNECT", "https://"+addr, nil); err != nil {
		return
	}
	if proxyAuth != "" {
		req.Header.Set("Proxy-Authorization", proxyAuth)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ngrok-plus)")
	if err = req.Write(s); err != nil {
		return
	}
	// read the proxy's response
	var res *http.Response
	if res, err = http.ReadResponse(bufio.NewReader(s), req); err != nil {
		return
	}
	if err = res.Body.Close(); err != nil {
		return
	}
	if res.StatusCode != http.StatusOK {
		err = errors.New(fmt.Sprintf("Non-200 response from proxy server: %s", res.Status))
		return
	}
	//upgrade to TLS
	s.StartTLS(tlsCfg)
	return
}

// Join 合并链接服务
func Join(c1 Conn, c2 Conn) (fromBytes, toBytes int64) {
	var wait sync.WaitGroup
	//
	pipe := func(to Conn, from Conn, bytesCopied *int64) {
		defer func(to Conn) {
			_ = to.Close()
		}(to)
		defer func(from Conn) {
			_ = from.Close()
		}(from)
		defer wait.Done()
		//
		var err error
		if *bytesCopied, err = io.Copy(to, from); err != nil {
			_ = from.Warn("Copied %d bytes to %s before failing with error %v", *bytesCopied, to.Id(), err)
		} else {
			from.Debug("Copied %d bytes to %s", *bytesCopied, to.Id())
		}
	}
	wait.Add(2)
	go pipe(c1, c2, &fromBytes)
	go pipe(c2, c1, &toBytes)
	c1.Info("Joined with connection %s", c2.Id())
	wait.Wait()
	return
}

func (c *loggedConn) StartTLS(tlsCfg *tls.Config) {
	c.Conn = tls.Client(c.Conn, tlsCfg)
}

func (c *loggedConn) Close() (err error) {
	if err = c.Conn.Close(); err == nil {
		c.Debug("Closing")
	}
	return
}

func (c *loggedConn) Id() string {
	return fmt.Sprintf("%s:%x", c.typ, c.id)
}

func (c *loggedConn) SetType(typ string) {
	oldId := c.id
	c.typ = typ
	c.Info("Renamed connection %s", oldId)
}

func (c *loggedConn) CloseRead() error {
	// XXX: use CloseRead() in Conn.Join() and in Control.shutdown() for cleaner
	// connection termination. Unfortunately, when I've tried that, I've observed
	// failures where the connection was closed *before* flushing its write buffer,
	// set with SetLinger() set properly (which it is by default).
	return c.tcp.CloseRead()
}
