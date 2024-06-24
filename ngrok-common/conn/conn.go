package conn

import (
	"crypto/tls"
	"github.com/gogf/gf/v2/os/glog"
	"net"
)

// Conn 连接接口
type Conn interface {
	net.Conn
	glog.ILogger
	// Id 链接ID
	Id() string
	// SetType 设置类型
	SetType(typ string)
	// StartTLS 开启TLS
	StartTLS(tlsCfg *tls.Config)
	// CloseRead 关闭读处理
	CloseRead() error
	// Close 关闭
	Close() (err error)
}

// Listener 监听服务
type Listener struct {
	net.Addr
	Connections chan Conn
}
