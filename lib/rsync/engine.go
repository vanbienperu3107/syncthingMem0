// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package rsync

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
)

const maxDataOperationSize = 1024 * 1024

// Engine contains reusable buffers for rsync operations.
type Engine struct {
	buffer []byte
}

// NewEngine creates an rsync engine.
func NewEngine(bufferSize int) *Engine {
	if bufferSize < 1 {
		bufferSize = maxBlockSize
	}
	return &Engine{
		buffer: make([]byte, bufferSize),
	}
}

// GenerateSignature reads the base data and returns hashes for each block.
func (e *Engine) GenerateSignature(reader io.Reader, fileSize uint64) (*Signature, error) {
	blockSize := OptimalBlockSizeForBaseLength(fileSize)
	sig := &Signature{BlockSize: blockSize}

	buf := make([]byte, blockSize)
	for {
		n, err := io.ReadFull(reader, buf)
		if n > 0 {
			block := buf[:n]
			rh := NewRollingHash(n)
			rh.Write(block)

			sig.Hashes = append(sig.Hashes, BlockHash{
				WeakHash:   rh.Sum(),
				StrongHash: sha1.Sum(block),
			})
			sig.LastBlockSize = uint64(n)
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	if len(sig.Hashes) == 0 {
		sig.LastBlockSize = 0
	}
	return sig, nil
}

// Deltify compares reader data with sig and emits a delta operation stream.
func (e *Engine) Deltify(reader io.Reader, sig *Signature, opHandler func(Operation) error) error {
	if err := validateSignature(sig); err != nil {
		return err
	}
	if len(sig.Hashes) == 0 {
		return e.emitAllAsData(reader, opHandler)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}

	weakMap := make(map[uint32][]int, len(sig.Hashes))
	for i, h := range sig.Hashes {
		weakMap[h.WeakHash] = append(weakMap[h.WeakHash], i)
	}

	var pending []byte
	var pendingBlock *Operation
	flushBlock := func() error {
		if pendingBlock == nil {
			return nil
		}
		err := opHandler(*pendingBlock)
		pendingBlock = nil
		return err
	}
	emitData := func(data []byte) error {
		if err := flushBlock(); err != nil {
			return err
		}
		for len(data) > 0 {
			n := len(data)
			if n > maxDataOperationSize {
				n = maxDataOperationSize
			}
			if err := opHandler(Operation{Type: OpData, Data: cloneBytes(data[:n])}); err != nil {
				return err
			}
			data = data[n:]
		}
		return nil
	}
	flushPending := func() error {
		if len(pending) == 0 {
			return nil
		}
		err := emitData(pending)
		pending = nil
		return err
	}
	emitBlock := func(blockIndex int) error {
		if err := flushPending(); err != nil {
			return err
		}
		if pendingBlock != nil && pendingBlock.BlockStart+pendingBlock.BlockCount == uint64(blockIndex) {
			pendingBlock.BlockCount++
			return nil
		}
		if err := flushBlock(); err != nil {
			return err
		}
		pendingBlock = &Operation{
			Type:       OpBlock,
			BlockStart: uint64(blockIndex),
			BlockCount: 1,
		}
		return nil
	}

	blockSize := int(sig.BlockSize)
	pos := 0
	for pos < len(data) {
		if shortLastBlockMatchPossible(pos, len(data), sig) {
			lastSize := int(sig.LastBlockSize)
			if idx, ok := e.matchBlock(data[pos:pos+lastSize], weakMap, sig); ok {
				if err := emitBlock(idx); err != nil {
					return err
				}
				pos += lastSize
				continue
			}
		}
		if len(data)-pos < blockSize {
			pending = append(pending, data[pos:]...)
			break
		}

		rh := NewRollingHash(blockSize)
		rh.Write(data[pos : pos+blockSize])
		for {
			if idx, ok := e.matchBlockWithWeak(data[pos:pos+blockSize], rh.Sum(), weakMap, sig); ok {
				if err := emitBlock(idx); err != nil {
					return err
				}
				pos += blockSize
				break
			}

			pending = append(pending, data[pos])
			pos++
			if len(data)-pos < blockSize {
				break
			}
			rh.Roll(data[pos+blockSize-1])
		}
	}

	if err := flushPending(); err != nil {
		return err
	}
	return flushBlock()
}

func (e *Engine) matchBlock(data []byte, weakMap map[uint32][]int, sig *Signature) (int, bool) {
	rh := NewRollingHash(len(data))
	rh.Write(data)
	return e.matchBlockWithWeak(data, rh.Sum(), weakMap, sig)
}

func (e *Engine) matchBlockWithWeak(data []byte, weak uint32, weakMap map[uint32][]int, sig *Signature) (int, bool) {
	candidates, ok := weakMap[weak]
	if !ok {
		return 0, false
	}

	strong := sha1.Sum(data)
	for _, idx := range candidates {
		if blockLength(sig, idx) == uint64(len(data)) && sig.Hashes[idx].StrongHash == strong {
			return idx, true
		}
	}
	return 0, false
}

// Patch applies delta operations to base and writes the reconstructed file.
func (e *Engine) Patch(base io.ReadSeeker, ops []Operation, blockSize uint64, output io.Writer) error {
	if blockSize == 0 {
		return errors.New("block size must be non-zero")
	}

	baseSize, err := seekSize(base)
	if err != nil {
		return err
	}
	blockCount := uint64(0)
	if baseSize > 0 {
		blockCount = (uint64(baseSize) + blockSize - 1) / blockSize
	}

	buf := e.buffer
	if uint64(len(buf)) < blockSize {
		buf = make([]byte, blockSize)
	}

	for _, op := range ops {
		switch op.Type {
		case OpData:
			if _, err := output.Write(op.Data); err != nil {
				return err
			}
		case OpBlock:
			if op.BlockCount == 0 {
				return errors.New("block operation has zero block count")
			}
			if op.BlockStart >= blockCount || op.BlockCount > blockCount-op.BlockStart {
				return fmt.Errorf("block operation references base blocks [%d, %d) beyond %d blocks", op.BlockStart, op.BlockStart+op.BlockCount, blockCount)
			}
			for i := uint64(0); i < op.BlockCount; i++ {
				blockIndex := op.BlockStart + i
				offset := int64(blockIndex * blockSize)
				size := blockSize
				if remaining := uint64(baseSize) - uint64(offset); remaining < size {
					size = remaining
				}
				if _, err := base.Seek(offset, io.SeekStart); err != nil {
					return err
				}
				if _, err := io.ReadFull(base, buf[:size]); err != nil {
					return err
				}
				if _, err := output.Write(buf[:size]); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unknown rsync operation type %d", op.Type)
		}
	}
	return nil
}

func (e *Engine) emitAllAsData(reader io.Reader, opHandler func(Operation) error) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	for len(data) > 0 {
		n := len(data)
		if n > maxDataOperationSize {
			n = maxDataOperationSize
		}
		if err := opHandler(Operation{Type: OpData, Data: cloneBytes(data[:n])}); err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

func validateSignature(sig *Signature) error {
	if sig == nil {
		return errors.New("nil signature")
	}
	if sig.BlockSize == 0 {
		return errors.New("signature block size must be non-zero")
	}
	if sig.LastBlockSize > sig.BlockSize {
		return errors.New("signature last block size exceeds block size")
	}
	if len(sig.Hashes) > 0 && sig.LastBlockSize == 0 {
		return errors.New("signature with hashes has zero last block size")
	}
	return nil
}

func blockLength(sig *Signature, index int) uint64 {
	if index == len(sig.Hashes)-1 {
		return sig.LastBlockSize
	}
	return sig.BlockSize
}

func shortLastBlockMatchPossible(pos, dataLength int, sig *Signature) bool {
	return sig.LastBlockSize > 0 &&
		sig.LastBlockSize < sig.BlockSize &&
		uint64(dataLength-pos) >= sig.LastBlockSize
}

func seekSize(r io.ReadSeeker) (int64, error) {
	current, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}
	if _, err := r.Seek(current, io.SeekStart); err != nil {
		return 0, err
	}
	return size, nil
}

func cloneBytes(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	clone := make([]byte, len(data))
	copy(clone, data)
	return clone
}
