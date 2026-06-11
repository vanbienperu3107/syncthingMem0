// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package connections

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
)

func TestWSConnReadWrite(t *testing.T) {
	withEchoWSConn(t, func(conn *wsConn) {
		data := []byte("hello syncthing")
		n, err := conn.Write(data)
		if err != nil {
			t.Fatal(err)
		}
		if n != len(data) {
			t.Fatalf("Write n=%d, want %d", n, len(data))
		}

		got := make([]byte, len(data))
		if _, err := io.ReadFull(conn, got); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, data) {
			t.Fatalf("got %q, want %q", got, data)
		}
	})
}

func TestWSConnMultipleMessages(t *testing.T) {
	withEchoWSConn(t, func(conn *wsConn) {
		if _, err := conn.Write([]byte("hello")); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Write([]byte("world")); err != nil {
			t.Fatal(err)
		}

		got := make([]byte, len("helloworld"))
		if _, err := io.ReadFull(conn, got); err != nil {
			t.Fatal(err)
		}
		if string(got) != "helloworld" {
			t.Fatalf("got %q, want %q", got, "helloworld")
		}
	})
}

func TestWSConnLargeMessage(t *testing.T) {
	withEchoWSConn(t, func(conn *wsConn) {
		data := bytes.Repeat([]byte("x"), 1<<20)
		if _, err := conn.Write(data); err != nil {
			t.Fatal(err)
		}

		got := make([]byte, len(data))
		if _, err := io.ReadFull(conn, got); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, data) {
			t.Fatal("large message mismatch")
		}
	})
}

func TestWSConnConcurrentReadWrite(t *testing.T) {
	withEchoWSConn(t, func(conn *wsConn) {
		const count = 100

		var wg sync.WaitGroup
		errs := make(chan error, 2)
		wg.Go(func() {
			for range count {
				if _, err := conn.Write([]byte("ping")); err != nil {
					errs <- err
					return
				}
			}
		})
		wg.Go(func() {
			buf := make([]byte, 4)
			for range count {
				if _, err := io.ReadFull(conn, buf); err != nil {
					errs <- err
					return
				}
				if string(buf) != "ping" {
					errs <- io.ErrUnexpectedEOF
					return
				}
			}
		})
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Fatal(err)
		}
	})
}

func TestFixupWSURI(t *testing.T) {
	cases := map[string]string{
		"wss://hub.example.com":		"wss://hub.example.com:443/ws",
		"wss://hub.example.com/sync":	"wss://hub.example.com:443/sync",
		"ws://127.0.0.1":		"ws://127.0.0.1:80/ws",
	}

	for raw, expected := range cases {
		uri, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if actual := fixupWSURI(uri).String(); actual != expected {
			t.Fatalf("fixupWSURI(%q) = %q, want %q", raw, actual, expected)
		}
	}
}

func withEchoWSConn(t *testing.T, test func(*wsConn)) {
	t.Helper()

	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer ws.Close()

		for {
			mt, msg, err := ws.ReadMessage()
			if err != nil {
				return
			}
			if err := ws.WriteMessage(mt, msg); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws://" + srv.Listener.Addr().String()
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	conn := newWSConn(ws)
	defer conn.Close()

	if _, ok := interface{}(conn).(net.Conn); !ok {
		t.Fatal("wsConn does not implement net.Conn")
	}

	test(conn)
}
