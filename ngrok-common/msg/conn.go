package msg

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
)

func readMsgShared(c service.ConnService) (buffer []byte, err error) {
	c.Debug(context.TODO(), "Waiting to read message")
	var sz int64
	if err = binary.Read(c, binary.LittleEndian, &sz); err != nil {
		return
	}
	c.Debugf(context.TODO(), "Reading message with length: %d", sz)
	buffer = make([]byte, sz)
	var n int
	n, err = c.Read(buffer)
	c.Debugf(context.TODO(), "Read message %s", buffer)
	if err != nil {
		return
	}
	if int64(n) != sz {
		err = errors.New(fmt.Sprintf("Expected to read %d bytes,but only read %d", sz, n))
		return
	}
	return
}

// ReadMsg 读取消息
func ReadMsg(c service.ConnService) (msg Message, err error) {
	var buf []byte
	if buf, err = readMsgShared(c); err != nil {
		return
	}
	return Unpack(buf)
}

// ReadMsgInfo 读取消息
func ReadMsgInfo(c service.ConnService, msg Message) (err error) {
	var buf []byte
	if buf, err = readMsgShared(c); err != nil {
		return
	}
	return UnpackInto(buf, msg)
}

func WriteMsg(c service.ConnService, msg interface{}) (err error) {
	var buf []byte
	if buf, err = Pack(msg); err != nil {
		return
	}
	c.Debugf(context.TODO(), "Writing message: %s", string(buf))
	if err = binary.Write(c, binary.LittleEndian, int64(len(buf))); err != nil {
		return
	}
	if _, err = c.Write(buf); err != nil {
		return
	}
	return nil
}
