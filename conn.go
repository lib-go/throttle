package throttle

import (
	"net"
)

type Conn struct {
	net.Conn
	read  ReadWriteFunc
	write ReadWriteFunc
}

func (c *Conn) SetReadSpeedLimit(kbps int, key string) {
	if kbps > 0 {
		c.read = SleepLimit(c.Conn.Read, kbps*1024, key)
	} else {
		c.read = nil
	}
}

func (c *Conn) SetWriteSpeedLimit(kbps int, key string) {
	if kbps > 0 {
		c.write = SleepLimit(c.Conn.Write, kbps*1024, key)
	} else {
		c.write = nil
	}
}

func (c Conn) Read(b []byte) (n int, e error) {
	if c.read == nil {
		return c.Conn.Read(b)
	} else {
		return c.read(b)
	}
}

func (c Conn) Write(b []byte) (n int, e error) {
	if c.write == nil {
		return c.Conn.Write(b)
	} else {
		return c.write(b)
	}
}
