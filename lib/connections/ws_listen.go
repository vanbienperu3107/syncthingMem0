// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package connections

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"

	"github.com/syncthing/syncthing/internal/slogutil"
	"github.com/syncthing/syncthing/lib/config"
	"github.com/syncthing/syncthing/lib/connections/registry"
	"github.com/syncthing/syncthing/lib/svcutil"
)

func init() {
	factory := &wsListenerFactory{}
	listeners["wss"] = factory
}

type wsListener struct {
	svcutil.ServiceWithError
	onAddressesChangedNotifier

	uri        *url.URL
	cfg        config.Wrapper
	tlsCfg     *tls.Config
	conns      chan internalConn
	factory    listenerFactory
	registry   *registry.Registry
	lanChecker *lanChecker

	laddr net.Addr
	mut   sync.RWMutex
}

func (l *wsListener) serve(ctx context.Context) error {
	tcpListener, err := net.Listen("tcp", l.uri.Host)
	if err != nil {
		slog.WarnContext(ctx, "Failed to listen (WSS)", slogutil.Error(err))
		return err
	}
	defer tcpListener.Close()

	l.mut.Lock()
	l.laddr = tcpListener.Addr()
	l.mut.Unlock()
	defer func() {
		l.mut.Lock()
		l.laddr = nil
		l.mut.Unlock()
	}()

	l.notifyAddressesChanged(l)
	defer l.clearAddresses(l)

	l.registry.Register(l.uri.Scheme, tcpListener.Addr())
	defer l.registry.Unregister(l.uri.Scheme, tcpListener.Addr())

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l.handleUpgrade(ctx, w, r)
	})
	server := &http.Server{
		Handler:   handler,
		TLSConfig: cloneTLSConfig(l.tlsCfg),
	}

	done := make(chan error, 1)
	go func() {
		var err error
		if l.uri.Scheme == "wss" {
			err = server.ServeTLS(tcpListener, "", "")
		} else {
			err = server.Serve(tcpListener)
		}
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		done <- err
	}()

	select {
	case <-ctx.Done():
		_ = server.Shutdown(context.Background())
		return nil
	case err := <-done:
		return err
	}
}

func (l *wsListener) handleUpgrade(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	path := l.uri.EscapedPath()
	if path == "" {
		path = "/ws"
	}
	if r.URL.EscapedPath() != path {
		http.NotFound(w, r)
		return
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  64 * 1024,
		WriteBufferSize: 64 * 1024,
		CheckOrigin:     func(*http.Request) bool { return true },
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	conn := newWSConn(ws)
	priority := l.cfg.Options().ConnectionPriorityTCPWAN
	isLocal := l.lanChecker.isLAN(conn.RemoteAddr())
	if isLocal {
		priority = l.cfg.Options().ConnectionPriorityTCPLAN
	}

	select {
	case l.conns <- newInternalConn(conn, connTypeWSSServer, isLocal, priority):
	case <-ctx.Done():
		_ = conn.Close()
	}
}

func (l *wsListener) URI() *url.URL {
	return l.uri
}

func (l *wsListener) WANAddresses() []*url.URL {
	l.mut.RLock()
	defer l.mut.RUnlock()
	return []*url.URL{maybeReplacePort(l.uri, l.laddr)}
}

func (l *wsListener) LANAddresses() []*url.URL {
	l.mut.RLock()
	uri := maybeReplacePort(l.uri, l.laddr)
	l.mut.RUnlock()
	addrs := []*url.URL{uri}
	addrs = append(addrs, getURLsForAllAdaptersIfUnspecified("tcp", uri)...)
	return addrs
}

func (l *wsListener) String() string {
	return l.uri.String()
}

func (l *wsListener) Factory() listenerFactory {
	return l.factory
}

func (*wsListener) NATType() string {
	return "unknown"
}

type wsListenerFactory struct{}

func (f *wsListenerFactory) New(uri *url.URL, cfg config.Wrapper, tlsCfg *tls.Config, conns chan internalConn, registry *registry.Registry, lanChecker *lanChecker) genericListener {
	uriCopy := *fixupWSURI(uri)
	l := &wsListener{
		uri:        &uriCopy,
		cfg:        cfg,
		tlsCfg:     tlsCfg,
		conns:      conns,
		factory:    f,
		registry:   registry,
		lanChecker: lanChecker,
	}
	l.ServiceWithError = svcutil.AsService(l.serve, l.String())
	return l
}

func (wsListenerFactory) Valid(_ config.Configuration) error {
	return nil
}
