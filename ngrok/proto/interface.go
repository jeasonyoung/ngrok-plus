package proto

import "ngrok-plus/ngrok/conn"

type Protocol interface {
	GetName() string
	WrapConn(conn.Conn, interface{}) conn.Conn
}
