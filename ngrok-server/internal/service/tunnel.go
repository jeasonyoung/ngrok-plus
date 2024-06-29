package service

import (
	"fmt"
	"net"
	"ngrok-common/log"
	"ngrok-common/msg"
	"ngrok-server/internal/consts"
	"os"
	"strings"
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
	// Register for spcific hostname
	hostname := strings.ToLower(strings.TrimSpace(t.req.Hostname))
	if hostname != "" {
		t.url = fmt.Sprintf("%s://%s", protocol, hostname)
		///TODO:
		return
	}

	///TODO:
	return
}

func NewTunnel(m *msg.ReqTunnel, ctl *Control) (t *Tunnel, err error) {
	///TODO:
	return
}

func (t *Tunnel) Shutdown() {
	///TODO:
	return
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
