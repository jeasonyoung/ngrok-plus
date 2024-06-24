package proto

import "ngrok-common/conn"

// Protocol 协议接口
type Protocol interface {
	GetName() string
	WrapConn(conn.Conn, interface{}) conn.Conn
}
