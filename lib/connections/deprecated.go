// Copyright (C) 2018 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package connections

import "github.com/syncthing/syncthing/lib/config"

// invalidListener is never valid
type invalidListener struct {
	listenerFactory

	err error
}

func (i invalidListener) Valid(_ config.Configuration) error {
	if i.err == nil {
		// fallback so we don't accidentally return nil
		return errUnsupported
	}
	return i.err
}

// invalidDialer is never valid
type invalidDialer struct {
	dialerFactory

	err error
}

func (i invalidDialer) Valid(_ config.Configuration) error {
	if i.err == nil {
		// fallback so we don't accidentally return nil
		return errUnsupported
	}
	return i.err
}

func init() {
	listeners["kcp"] = invalidListener{err: errDeprecated}
	listeners["kcp4"] = invalidListener{err: errDeprecated}
	listeners["kcp6"] = invalidListener{err: errDeprecated}
	dialers["kcp"] = invalidDialer{err: errDeprecated}
	dialers["kcp4"] = invalidDialer{err: errDeprecated}
	dialers["kcp6"] = invalidDialer{err: errDeprecated}

	// TCP, QUIC, relay removed in syncthingMem0 (WSS-only hub model)
	listeners["tcp"] = invalidListener{err: errDeprecated}
	listeners["tcp4"] = invalidListener{err: errDeprecated}
	listeners["tcp6"] = invalidListener{err: errDeprecated}
	listeners["quic"] = invalidListener{err: errDeprecated}
	listeners["quic4"] = invalidListener{err: errDeprecated}
	listeners["quic6"] = invalidListener{err: errDeprecated}
	listeners["relay"] = invalidListener{err: errDeprecated}
	listeners["dynamic+http"] = invalidListener{err: errDeprecated}
	listeners["dynamic+https"] = invalidListener{err: errDeprecated}
	dialers["tcp"] = invalidDialer{err: errDeprecated}
	dialers["tcp4"] = invalidDialer{err: errDeprecated}
	dialers["tcp6"] = invalidDialer{err: errDeprecated}
	dialers["quic"] = invalidDialer{err: errDeprecated}
	dialers["quic4"] = invalidDialer{err: errDeprecated}
	dialers["quic6"] = invalidDialer{err: errDeprecated}
	dialers["relay"] = invalidDialer{err: errDeprecated}
}
