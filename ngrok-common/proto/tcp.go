package proto

import "ngrok-common/conn"

type Tcp struct {
}

func NewTcp() *Tcp {
	return new(Tcp)
}

func (t *Tcp) GetName() string {
	return "tcp"
}

func (t *Tcp) WrapConn(c conn.Conn, ctx interface{}) conn.Conn {
	return c
}
