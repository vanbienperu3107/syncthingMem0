// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

// Package rsync implements the core rsync delta transfer algorithm.
package rsync

import "math/bits"

const (
	minBlockSize = 1024
	maxBlockSize = 64 * 1024
	maxUint64    = ^uint64(0)
)

// OperationType describes the kind of delta operation.
type OperationType int

const (
	// OpBlock references one or more consecutive blocks from the base file.
	OpBlock OperationType = iota

	// OpData contains new bytes that were not found in the base signature.
	OpData
)

// Operation is one item in a delta stream.
type Operation struct {
	Type OperationType

	BlockStart uint64
	BlockCount uint64

	Data []byte
}

// BlockHash stores the weak and strong hashes for one base-file block.
type BlockHash struct {
	WeakHash   uint32
	StrongHash [20]byte
}

// Signature describes the blocks present in a base file.
type Signature struct {
	BlockSize     uint64
	LastBlockSize uint64
	Hashes        []BlockHash
}

// OptimalBlockSizeForBaseLength returns an rsync block size for a base file.
func OptimalBlockSizeForBaseLength(length uint64) uint64 {
	if length == 0 {
		return minBlockSize
	}
	if length > maxUint64/24 {
		return maxBlockSize
	}

	optimal := isqrt(length * 24)
	if optimal < minBlockSize {
		return minBlockSize
	}
	if optimal > maxBlockSize {
		return maxBlockSize
	}
	return optimal
}

func isqrt(n uint64) uint64 {
	if n == 0 {
		return 0
	}

	x := uint64(1) << ((bits.Len64(n) + 1) / 2)
	for {
		y := (x + n/x) / 2
		if y >= x {
			return x
		}
		x = y
	}
}
