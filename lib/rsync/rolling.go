// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package rsync

// RollingHash is the rsync weak checksum for a fixed-size sliding window.
type RollingHash struct {
	a         uint16
	b         uint16
	blockSize int
	window    []byte
	pos       int
}

// NewRollingHash creates a rolling hash with the given window size.
func NewRollingHash(blockSize int) *RollingHash {
	return &RollingHash{
		blockSize: blockSize,
		window:    make([]byte, blockSize),
	}
}

// Write initializes the checksum from data.
func (r *RollingHash) Write(data []byte) {
	r.a = 0
	r.b = 0
	r.pos = 0

	for i := range r.window {
		r.window[i] = 0
	}
	for i, v := range data {
		r.a += uint16(v)
		r.b += uint16(len(data)-i) * uint16(v)
		if i < len(r.window) {
			r.window[i] = v
		}
	}
}

// Roll advances the window by one byte.
func (r *RollingHash) Roll(in byte) {
	out := r.window[r.pos]

	r.a += uint16(in) - uint16(out)
	r.b -= uint16(r.blockSize) * uint16(out)
	r.b += r.a

	r.window[r.pos] = in
	r.pos = (r.pos + 1) % r.blockSize
}

// Sum returns the current 32-bit weak checksum.
func (r *RollingHash) Sum() uint32 {
	return uint32(r.a) | uint32(r.b)<<16
}

// Reset clears the rolling hash state.
func (r *RollingHash) Reset() {
	r.a = 0
	r.b = 0
	r.pos = 0
	for i := range r.window {
		r.window[i] = 0
	}
}
