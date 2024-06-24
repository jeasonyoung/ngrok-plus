package conn

import (
	"bufio"
	"io"
)

type teeLogic struct {
	rd       io.Reader
	wr       io.Writer
	readPipe struct {
		rd *io.PipeReader
		wr *io.PipeWriter
	}
	writePipe struct {
		rd *io.PipeReader
		wr *io.PipeWriter
	}
	Conn
}

// NewTee 构建对象实例
func NewTee(s Conn) Tee {
	c := &teeLogic{
		rd:   nil,
		wr:   nil,
		Conn: s,
	}
	c.readPipe.rd, c.readPipe.wr = io.Pipe()
	c.writePipe.rd, c.writePipe.wr = io.Pipe()
	c.rd = io.TeeReader(c.Conn, c.readPipe.wr)
	c.wr = io.MultiWriter(c.Conn, c.writePipe.wr)
	return c
}

func (c *teeLogic) ReadBuffer() *bufio.Reader {
	return bufio.NewReader(c.readPipe.rd)
}

func (c *teeLogic) WriteBuffer() *bufio.Reader {
	return bufio.NewReader(c.writePipe.rd)
}

func (c *teeLogic) Read(b []byte) (n int, err error) {
	if n, err = c.rd.Read(b); err != nil {
		_ = c.readPipe.wr.Close()
	}
	return
}

func (c *teeLogic) ReadFrom(r io.Reader) (n int64, err error) {
	if n, err = io.Copy(c.wr, r); err != nil {
		_ = c.writePipe.wr.Close()
	}
	return
}

func (c *teeLogic) Write(b []byte) (n int, err error) {
	if n, err = c.wr.Write(b); err != nil {
		_ = c.writePipe.wr.Close()
	}
	return
}
