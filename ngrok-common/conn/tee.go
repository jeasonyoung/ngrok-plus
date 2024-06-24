package conn

import (
	"bufio"
	"io"
)

type Tee interface {
	Conn
	ReadBuffer() *bufio.Reader
	WriteBuffer() *bufio.Reader
	Read(b []byte) (n int, err error)
	ReadFrom(r io.Reader) (n int64, err error)
	Write(b []byte) (n int, err error)
}
