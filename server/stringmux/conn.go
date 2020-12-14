package stringmux

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/juju/errors"
	"github.com/peer-calls/peer-calls/server/logger"
)

type Conn interface {
	net.Conn
	StreamID() string
	CloseChannel() <-chan struct{}
}

type conn struct {
	debugLogger logger.Logger

	conn     net.Conn
	streamID string

	readChan             chan []byte
	closeConnRequestChan chan closeConnRequest
	torndown             chan struct{}
}

var _ Conn = &conn{}

func (c *conn) StreamID() string {
	return c.streamID
}

func (c *conn) Close() error {
	req := closeConnRequest{
		conn:    c,
		errChan: make(chan error, 1),
	}

	select {
	case c.closeConnRequestChan <- req:
	case <-c.torndown:
		return nil
	}

	err := <-req.errChan

	return errors.Trace(err)
}

func (c *conn) close() {
	close(c.readChan)
	close(c.torndown)
}

func (c *conn) CloseChannel() <-chan struct{} {
	return c.torndown
}

func (c *conn) Read(b []byte) (int, error) {
	buf, ok := <-c.readChan
	if !ok {
		return 0, errors.Annotate(io.ErrClosedPipe, "read")
	}

	copy(b, buf)

	c.debugLogger.Printf("%s recv %v", c, buf)

	return len(buf), nil
}

func (c *conn) Write(b []byte) (int, error) {
	select {
	case <-c.torndown:
		return 0, errors.Annotate(io.ErrClosedPipe, "write")
	default:
		data, err := Marshal(c.streamID, b)
		if err != nil {
			return 0, errors.Annotate(err, "marshal during write")
		}

		c.debugLogger.Printf("%s send %v", c, b)

		_, err = c.conn.Write(data)

		if err != nil {
			return 0, errors.Annotate(err, "conn write")
		}

		return len(b), nil
	}
}

func (c *conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *conn) SetDeadline(t time.Time) error {
	return nil
}

func (c *conn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *conn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (c *conn) String() string {
	if s, ok := c.conn.(stringer); ok {
		return fmt.Sprintf("%s [%s]", s.String(), c.streamID)
	}

	return fmt.Sprintf("[%s]", c.streamID)
}

type stringer interface {
	String() string
}
