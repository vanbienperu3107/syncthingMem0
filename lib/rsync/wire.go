// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package rsync

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// This file provides a transport-independent, in-memory delta API meant to be
// wired into the block transfer path later. The wire representation (signature
// and delta byte blobs) is deliberately opaque so it can be carried inside a
// protocol message field without the protocol layer knowing rsync internals.
//
// Typical per-block use:
//
//	requester (has old block O, wants new block N):
//	    sig := SignatureBytes(O)          // send sig to peer
//	responder (has new block N):
//	    delta := DeltaBytes(N, sig)       // send delta back
//	requester:
//	    N2 := ApplyDelta(O, delta)        // N2 == N, write at offset

const (
	sigCodecVersion   byte = 1
	deltaCodecVersion byte = 1
)

var (
	errCorruptSignature = errors.New("corrupt rsync signature")
	errCorruptDelta     = errors.New("corrupt rsync delta")
)

// SignatureBytes computes and encodes an rsync signature over base, the local
// (old) version. It is run on the side that wants to receive the new version.
func SignatureBytes(base []byte) ([]byte, error) {
	e := NewEngine(0)
	sig, err := e.GenerateSignature(bytes.NewReader(base), uint64(len(base)))
	if err != nil {
		return nil, err
	}
	return encodeSignature(sig), nil
}

// DeltaBytes computes an encoded delta that reconstructs target from the base
// described by sigBytes. It is run on the side that holds the new version.
func DeltaBytes(target, sigBytes []byte) ([]byte, error) {
	sig, err := decodeSignature(sigBytes)
	if err != nil {
		return nil, err
	}
	e := NewEngine(0)
	var ops []Operation
	if err := e.Deltify(bytes.NewReader(target), sig, func(op Operation) error {
		ops = append(ops, op)
		return nil
	}); err != nil {
		return nil, err
	}
	return encodeDelta(sig.BlockSize, ops), nil
}

// ApplyDelta reconstructs the new version from the local base and an encoded
// delta produced by DeltaBytes. It is run on the side that holds the old
// version.
func ApplyDelta(base, deltaBytes []byte) ([]byte, error) {
	blockSize, ops, err := decodeDelta(deltaBytes)
	if err != nil {
		return nil, err
	}
	e := NewEngine(0)
	var out bytes.Buffer
	if err := e.Patch(bytes.NewReader(base), ops, blockSize, &out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func encodeSignature(sig *Signature) []byte {
	buf := make([]byte, 0, 16+len(sig.Hashes)*(4+len(BlockHash{}.StrongHash)))
	buf = append(buf, sigCodecVersion)
	buf = binary.AppendUvarint(buf, sig.BlockSize)
	buf = binary.AppendUvarint(buf, sig.LastBlockSize)
	buf = binary.AppendUvarint(buf, uint64(len(sig.Hashes)))
	var wh [4]byte
	for _, h := range sig.Hashes {
		binary.BigEndian.PutUint32(wh[:], h.WeakHash)
		buf = append(buf, wh[:]...)
		buf = append(buf, h.StrongHash[:]...)
	}
	return buf
}

func decodeSignature(b []byte) (*Signature, error) {
	r := bytes.NewReader(b)
	ver, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errCorruptSignature, err)
	}
	if ver != sigCodecVersion {
		return nil, fmt.Errorf("%w: unsupported version %d", errCorruptSignature, ver)
	}
	blockSize, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, fmt.Errorf("%w: block size", errCorruptSignature)
	}
	lastBlockSize, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, fmt.Errorf("%w: last block size", errCorruptSignature)
	}
	n, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, fmt.Errorf("%w: block count", errCorruptSignature)
	}
	// Each hash entry is 4 (weak) + len(StrongHash) bytes on the wire; reject
	// counts that cannot possibly fit in the remaining data so a hostile length
	// cannot force a huge allocation.
	perHash := uint64(4 + len(BlockHash{}.StrongHash))
	if n > uint64(r.Len())/perHash {
		return nil, fmt.Errorf("%w: block count %d exceeds data", errCorruptSignature, n)
	}
	sig := &Signature{
		BlockSize:     blockSize,
		LastBlockSize: lastBlockSize,
		Hashes:        make([]BlockHash, 0, n),
	}
	for i := uint64(0); i < n; i++ {
		var wh [4]byte
		if _, err := io.ReadFull(r, wh[:]); err != nil {
			return nil, fmt.Errorf("%w: weak hash", errCorruptSignature)
		}
		var sh [20]byte
		if _, err := io.ReadFull(r, sh[:]); err != nil {
			return nil, fmt.Errorf("%w: strong hash", errCorruptSignature)
		}
		sig.Hashes = append(sig.Hashes, BlockHash{
			WeakHash:   binary.BigEndian.Uint32(wh[:]),
			StrongHash: sh,
		})
	}
	return sig, nil
}

func encodeDelta(blockSize uint64, ops []Operation) []byte {
	buf := make([]byte, 0, 16)
	buf = append(buf, deltaCodecVersion)
	buf = binary.AppendUvarint(buf, blockSize)
	buf = binary.AppendUvarint(buf, uint64(len(ops)))
	for _, op := range ops {
		buf = append(buf, byte(op.Type))
		switch op.Type {
		case OpBlock:
			buf = binary.AppendUvarint(buf, op.BlockStart)
			buf = binary.AppendUvarint(buf, op.BlockCount)
		case OpData:
			buf = binary.AppendUvarint(buf, uint64(len(op.Data)))
			buf = append(buf, op.Data...)
		}
	}
	return buf
}

func decodeDelta(b []byte) (uint64, []Operation, error) {
	r := bytes.NewReader(b)
	ver, err := r.ReadByte()
	if err != nil {
		return 0, nil, fmt.Errorf("%w: %v", errCorruptDelta, err)
	}
	if ver != deltaCodecVersion {
		return 0, nil, fmt.Errorf("%w: unsupported version %d", errCorruptDelta, ver)
	}
	blockSize, err := binary.ReadUvarint(r)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: block size", errCorruptDelta)
	}
	n, err := binary.ReadUvarint(r)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: op count", errCorruptDelta)
	}
	// The smallest possible op is 2 bytes (type + one uvarint); reject counts
	// that cannot fit in the remaining data.
	if n > uint64(r.Len())/2 {
		return 0, nil, fmt.Errorf("%w: op count %d exceeds data", errCorruptDelta, n)
	}
	ops := make([]Operation, 0, n)
	for i := uint64(0); i < n; i++ {
		t, err := r.ReadByte()
		if err != nil {
			return 0, nil, fmt.Errorf("%w: op type", errCorruptDelta)
		}
		switch OperationType(t) {
		case OpBlock:
			start, err := binary.ReadUvarint(r)
			if err != nil {
				return 0, nil, fmt.Errorf("%w: block start", errCorruptDelta)
			}
			count, err := binary.ReadUvarint(r)
			if err != nil {
				return 0, nil, fmt.Errorf("%w: block count", errCorruptDelta)
			}
			ops = append(ops, Operation{Type: OpBlock, BlockStart: start, BlockCount: count})
		case OpData:
			size, err := binary.ReadUvarint(r)
			if err != nil {
				return 0, nil, fmt.Errorf("%w: data size", errCorruptDelta)
			}
			if size > maxDataOperationSize || size > uint64(r.Len()) {
				return 0, nil, fmt.Errorf("%w: data size %d", errCorruptDelta, size)
			}
			data := make([]byte, size)
			if _, err := io.ReadFull(r, data); err != nil {
				return 0, nil, fmt.Errorf("%w: data", errCorruptDelta)
			}
			ops = append(ops, Operation{Type: OpData, Data: data})
		default:
			return 0, nil, fmt.Errorf("%w: unknown op type %d", errCorruptDelta, t)
		}
	}
	return blockSize, ops, nil
}
