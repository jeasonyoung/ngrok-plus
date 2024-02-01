package conn

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	vhost "github.com/inconshreveable/go-vhost"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"ngrok-plus/ngrok/log"
	"sync"
)

type Conn interface {
	net.Conn
	log.Logger
	Id() string
	SetType(string)
	CloseRead() error
}

type LoggedConn struct {
	tcp *net.TCPConn
	net.Conn
	log.Logger
	id  int32
	typ string
}

type Listener struct {
	net.Addr
	Conns chan *LoggedConn
}

func wrapConn(conn net.Conn, typ string) *LoggedConn {
	switch c := conn.(type) {
	case *vhost.HTTPConn:
		wrapped := c.Conn.(*LoggedConn)
		return &LoggedConn{wrapped.tcp, conn, wrapped.Logger, wrapped.id, wrapped.typ}
	case *LoggedConn:
		return c
	case *net.TCPConn:
		wrapped := &LoggedConn{c, conn, log.NewPrefixLogger(), rand.Int31(), typ}
		wrapped.AddLogPrefix(wrapped.Id())
		return wrapped
	}

	return nil
}

func Listen(addr, typ string, tlsCfg *tls.Config) (l *Listener, err error) {
	// listen for incoming connections
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return
	}

	l = &Listener{
		Addr:  listener.Addr(),
		Conns: make(chan *LoggedConn),
	}

	go func() {
		for {
			var rawConn net.Conn
			rawConn, err = listener.Accept()
			if err != nil {
				_ = log.Error("Failed to accept new TCP connection of type %s: %v", typ, err)
				continue
			}

			c := wrapConn(rawConn, typ)
			if tlsCfg != nil {
				c.Conn = tls.Server(c.Conn, tlsCfg)
			}
			c.Info("New connection from %v", c.RemoteAddr())
			l.Conns <- c
		}
	}()
	return
}

func Wrap(conn net.Conn, typ string) *LoggedConn {
	return wrapConn(conn, typ)
}

func Dial(addr, typ string, tlsCfg *tls.Config) (conn *LoggedConn, err error) {
	var rawConn net.Conn
	if rawConn, err = net.Dial("tcp", addr); err != nil {
		return
	}

	conn = wrapConn(rawConn, typ)
	conn.Debug("New connection to: %v", rawConn.RemoteAddr())

	if tlsCfg != nil {
		conn.StartTLS(tlsCfg)
	}

	return
}

func DialHttpProxy(proxyUrl, addr, typ string, tlsCfg *tls.Config) (conn *LoggedConn, err error) {
	// parse the proxy address
	var parsedUrl *url.URL
	if parsedUrl, err = url.Parse(proxyUrl); err != nil {
		return
	}

	var proxyAuth string
	if parsedUrl.User != nil {
		proxyAuth = "Basic " + base64.StdEncoding.EncodeToString([]byte(parsedUrl.User.String()))
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
	if conn, err = Dial(parsedUrl.Host, typ, proxyTlsConfig); err != nil {
		return
	}

	// send an HTTP proxy CONNECT message
	req, err := http.NewRequest("CONNECT", "https://"+addr, nil)
	if err != nil {
		return
	}

	if proxyAuth != "" {
		req.Header.Set("Proxy-Authorization", proxyAuth)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ngrok)")
	_ = req.Write(conn)

	// read the proxy's response
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()

	if resp.StatusCode != 200 {
		err = fmt.Errorf("non-200 response from proxy server: %s", resp.Status)
		return
	}

	// upgrade to TLS
	conn.StartTLS(tlsCfg)

	return
}

func (c *LoggedConn) StartTLS(tlsCfg *tls.Config) {
	c.Conn = tls.Client(c.Conn, tlsCfg)
}

func (c *LoggedConn) Close() (err error) {
	if err := c.Conn.Close(); err == nil {
		c.Debug("Closing")
	}
	return
}

func (c *LoggedConn) Id() string {
	return fmt.Sprintf("%s:%x", c.typ, c.id)
}

func (c *LoggedConn) SetType(typ string) {
	oldId := c.Id()
	c.typ = typ
	c.ClearLogPrefixes()
	c.AddLogPrefix(c.Id())
	c.Info("Renamed connection %s", oldId)
}

func (c *LoggedConn) CloseRead() error {
	// XXX: use CloseRead() in Conn.Join() and in Control.shutdown() for cleaner
	// connection termination. Unfortunately, when I've tried that, I've observed
	// failures where the connection was closed *before* flushing its write buffer,
	// set with SetLinger() set properly (which it is by default).
	return c.tcp.CloseRead()
}

func Join(c Conn, c2 Conn) (int64, int64) {
	var wait sync.WaitGroup

	pipe := func(to Conn, from Conn, bytesCopied *int64) {
		defer func(to Conn) {
			err := to.Close()
			if err != nil {

			}
		}(to)
		defer func(from Conn) {
			err := from.Close()
			if err != nil {

			}
		}(from)
		defer wait.Done()

		var err error
		*bytesCopied, err = io.Copy(to, from)
		if err != nil {
			_ = from.Warn("copied %d bytes to %s before failing with error %v", *bytesCopied, to.Id(), err)
		} else {
			from.Debug("Copied %d bytes to %s", *bytesCopied, to.Id())
		}
	}

	wait.Add(2)
	var fromBytes, toBytes int64
	go pipe(c, c2, &fromBytes)
	go pipe(c2, c, &toBytes)
	c.Info("Joined with connection %s", c2.Id())
	wait.Wait()
	return fromBytes, toBytes
}
