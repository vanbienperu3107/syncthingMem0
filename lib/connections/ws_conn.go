// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package connections

import (
	"crypto/tls"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const maxWSMessageSize = 500 << 20

type wsConn struct {
	ws *websocket.Conn

	readMut sync.Mutex
	reader  io.Reader

	writeMut sync.Mutex
}

func newWSConn(ws *websocket.Conn) *wsConn {
	ws.SetReadLimit(maxWSMessageSize)
	return &wsConn{ws: ws}
}

func (c *wsConn) Read(p []byte) (int, error) {
	c.readMut.Lock()
	defer c.readMut.Unlock()

	for {
		if c.reader == nil {
			_, r, err := c.ws.NextReader()
			if err != nil {
				return 0, err
			}
			c.reader = r
		}

		n, err := c.reader.Read(p)
		if err == io.EOF {
			c.reader = nil
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

func (c *wsConn) Write(p []byte) (int, error) {
	c.writeMut.Lock()
	defer c.writeMut.Unlock()

	if err := c.ws.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *wsConn) Close() error {
	return c.ws.Close()
}

func (c *wsConn) LocalAddr() net.Addr {
	return c.ws.LocalAddr()
}

func (c *wsConn) RemoteAddr() net.Addr {
	return c.ws.RemoteAddr()
}

func (c *wsConn) SetDeadline(t time.Time) error {
	if err := c.ws.SetReadDeadline(t); err != nil {
		return err
	}
	return c.ws.SetWriteDeadline(t)
}

func (c *wsConn) SetReadDeadline(t time.Time) error {
	return c.ws.SetReadDeadline(t)
}

func (c *wsConn) SetWriteDeadline(t time.Time) error {
	return c.ws.SetWriteDeadline(t)
}

func (c *wsConn) ConnectionState() tls.ConnectionState {
	if tc, ok := c.ws.UnderlyingConn().(*tls.Conn); ok {
		return tc.ConnectionState()
	}
	return tls.ConnectionState{}
}
