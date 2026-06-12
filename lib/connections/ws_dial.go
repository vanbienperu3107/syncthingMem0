// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package connections

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"

	"github.com/syncthing/syncthing/lib/config"
	"github.com/syncthing/syncthing/lib/connections/registry"
	"github.com/syncthing/syncthing/lib/protocol"
)

func init() {
	factory := &wsDialerFactory{}
	dialers["wss"] = factory
}

type wsDialer struct {
	commonDialer
}

func (d *wsDialer) Dial(ctx context.Context, id protocol.DeviceID, uri *url.URL) (internalConn, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	uri = fixupWSURI(uri)

	header := http.Header{}
	header.Set("X-Device-ID", id.String())

	dialer := websocket.Dialer{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: cloneTLSConfig(d.tlsCfg),
	}
	if uri.Scheme == "wss" && dialer.TLSClientConfig.MinVersion < tls.VersionTLS13 {
		dialer.TLSClientConfig.MinVersion = tls.VersionTLS13
	}

	ws, _, err := dialer.DialContext(timeoutCtx, uri.String(), header)
	if err != nil {
		return internalConn{}, err
	}

	conn := newWSConn(ws)
	priority := d.wanPriority
	isLocal := d.lanChecker.isLAN(conn.RemoteAddr())
	if isLocal {
		priority = d.lanPriority
	}

	return newInternalConn(conn, connTypeWSSClient, isLocal, priority), nil
}

type wsDialerFactory struct{}

func (wsDialerFactory) New(opts config.OptionsConfiguration, tlsCfg *tls.Config, _ *registry.Registry, lanChecker *lanChecker) genericDialer {
	return &wsDialer{
		commonDialer: commonDialer{
			reconnectInterval: time.Duration(opts.ReconnectIntervalS) * time.Second,
			tlsCfg:            tlsCfg,
			lanChecker:        lanChecker,
			lanPriority:       opts.ConnectionPriorityTCPLAN,
			wanPriority:       opts.ConnectionPriorityTCPWAN,
			allowsMultiConns:  true,
		},
	}
}

func (wsDialerFactory) AlwaysWAN() bool {
	return false
}

func (wsDialerFactory) Valid(_ config.Configuration) error {
	return nil
}

func (wsDialerFactory) String() string {
	return "WSS Dialer"
}

func fixupWSURI(uri *url.URL) *url.URL {
	defaultPort := 80
	if uri.Scheme == "wss" {
		defaultPort = 443
	}
	uri = fixupPort(uri, defaultPort)
	if uri.Path != "" {
		return uri
	}
	uriCopy := *uri
	uriCopy.Path = "/ws"
	return &uriCopy
}

func cloneTLSConfig(cfg *tls.Config) *tls.Config {
	if cfg == nil {
		return &tls.Config{}
	}
	clone := cfg.Clone()
	clone.NextProtos = nil
	return clone
}
