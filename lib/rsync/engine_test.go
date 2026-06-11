// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package rsync

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"math/rand"
	"testing"
)

func TestRollingHashWriteVsRoll(t *testing.T) {
	data := []byte("Hello, rsync delta transfer algorithm!")
	blockSize := 8

	rh1 := NewRollingHash(blockSize)
	rh1.Write(data[len(data)-blockSize:])

	rh2 := NewRollingHash(blockSize)
	rh2.Write(data[:blockSize])
	for i := blockSize; i < len(data); i++ {
		rh2.Roll(data[i])
	}

	if rh1.Sum() != rh2.Sum() {
		t.Fatalf("Write() = %d, Roll() = %d", rh1.Sum(), rh2.Sum())
	}
}

func TestSignatureBlockCount(t *testing.T) {
	tests := []struct {
		name     string
		fileSize int
		wantMin  int
		wantMax  int
	}{
		{"1KB file", 1024, 1, 1},
		{"10KB file", 10 * 1024, 1, 15},
		{"1MB file", 1024 * 1024, 15, 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := deterministicBytes(tt.fileSize)
			engine := NewEngine(maxBlockSize)

			sig, err := engine.GenerateSignature(bytes.NewReader(data), uint64(tt.fileSize))
			if err != nil {
				t.Fatal(err)
			}

			if got := len(sig.Hashes); got < tt.wantMin || got > tt.wantMax {
				t.Fatalf("block count = %d, want in [%d, %d]", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestSignatureHashes(t *testing.T) {
	data := deterministicBytes(3*1024 + 100)
	engine := NewEngine(maxBlockSize)

	sig, err := engine.GenerateSignature(bytes.NewReader(data), uint64(len(data)))
	if err != nil {
		t.Fatal(err)
	}

	if len(sig.Hashes) < 2 {
		t.Fatalf("got %d hashes, want multiple blocks", len(sig.Hashes))
	}
	for i, hash := range sig.Hashes {
		offset := i * int(sig.BlockSize)
		end := offset + int(blockLength(sig, i))
		block := data[offset:end]

		rh := NewRollingHash(len(block))
		rh.Write(block)
		if hash.WeakHash != rh.Sum() {
			t.Fatalf("block %d weak hash = %d, want %d", i, hash.WeakHash, rh.Sum())
		}
		if hash.StrongHash != sha1.Sum(block) {
			t.Fatalf("block %d strong hash mismatch", i)
		}
	}
}

func TestDeltifyPatchIdenticalFiles(t *testing.T) {
	original := bytes.Repeat([]byte("Syncthing rsync delta test! "), 1000)
	ops, result, sig := deltifyPatch(t, original, original)

	for _, op := range ops {
		if op.Type == OpData {
			t.Fatalf("identical files emitted OpData of %d bytes", len(op.Data))
		}
	}
	if count := totalBlockRefs(ops); count != uint64(len(sig.Hashes)) {
		t.Fatalf("got %d block references, want %d", count, len(sig.Hashes))
	}
	if !bytes.Equal(result, original) {
		t.Fatal("patched output does not match original")
	}
}

func TestDeltifyPatchInsertAtStart(t *testing.T) {
	original := bytes.Repeat([]byte("ABCDEFGHIJKLMNOP"), 1000)
	modified := append([]byte{'X'}, original...)

	ops, result, _ := deltifyPatch(t, original, modified)

	if totalDataBytes(ops) > len(original)/10 {
		t.Fatalf("OpData = %d bytes, want less than %d", totalDataBytes(ops), len(original)/10)
	}
	if !bytes.Equal(result, modified) {
		t.Fatal("patched output does not match modified")
	}
}

func TestDeltifyPatchModifyMiddle(t *testing.T) {
	original := deterministicBytes(256 * 1024)
	modified := cloneBytes(original)
	copy(modified[100*1024:110*1024], bytes.Repeat([]byte{0xa5}, 10*1024))

	ops, result, sig := deltifyPatch(t, original, modified)

	if totalDataBytes(ops) > 10*1024+2*int(sig.BlockSize) {
		t.Fatalf("OpData = %d bytes, want close to the edited range", totalDataBytes(ops))
	}
	if !bytes.Equal(result, modified) {
		t.Fatal("patched output does not match modified")
	}
}

func TestDeltifyPatchDeleteMiddle(t *testing.T) {
	original := deterministicBytes(256 * 1024)
	modified := append(cloneBytes(original[:80*1024]), original[96*1024:]...)

	ops, result, sig := deltifyPatch(t, original, modified)

	if totalDataBytes(ops) > int(sig.BlockSize) {
		t.Fatalf("OpData = %d bytes, want at most one boundary block", totalDataBytes(ops))
	}
	if !bytes.Equal(result, modified) {
		t.Fatal("patched output does not match modified")
	}
}

func TestDeltifyPatchLastBlock(t *testing.T) {
	original := deterministicBytes(20*1024 + 17)
	modified := append([]byte("prefix"), original...)

	ops, result, sig := deltifyPatch(t, original, modified)

	if sig.LastBlockSize == sig.BlockSize {
		t.Fatal("test data should create a short final block")
	}
	if totalDataBytes(ops) > len("prefix")+int(sig.BlockSize) {
		t.Fatalf("last block was not matched efficiently; OpData=%d", totalDataBytes(ops))
	}
	if !bytes.Equal(result, modified) {
		t.Fatal("patched output does not match modified")
	}
}

func TestDeltifyPatchLastBlockBeforeSuffix(t *testing.T) {
	original := deterministicBytes(20*1024 + 17)
	modified := append(cloneBytes(original), []byte("suffix")...)

	ops, result, sig := deltifyPatch(t, original, modified)

	if sig.LastBlockSize == sig.BlockSize {
		t.Fatal("test data should create a short final block")
	}
	if totalDataBytes(ops) != len("suffix") {
		t.Fatalf("OpData = %d bytes, want suffix only", totalDataBytes(ops))
	}
	if !bytes.Equal(result, modified) {
		t.Fatal("patched output does not match modified")
	}
}

func TestDeltifyPatchEmptyBase(t *testing.T) {
	engine := NewEngine(maxBlockSize)
	sig, err := engine.GenerateSignature(bytes.NewReader(nil), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(sig.Hashes) != 0 {
		t.Fatalf("empty file has %d hashes, want 0", len(sig.Hashes))
	}

	newContent := []byte("Completely new file content")
	var ops []Operation
	if err := engine.Deltify(bytes.NewReader(newContent), sig, func(op Operation) error {
		ops = append(ops, op)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if len(ops) != 1 || ops[0].Type != OpData {
		t.Fatalf("got %d operations, want one data operation", len(ops))
	}
	if !bytes.Equal(ops[0].Data, newContent) {
		t.Fatal("data operation does not contain all new content")
	}
}

func TestDeltifyPatchEmptyTarget(t *testing.T) {
	ops, result, _ := deltifyPatch(t, deterministicBytes(16*1024), nil)
	if len(ops) != 0 {
		t.Fatalf("got %d operations, want none", len(ops))
	}
	if len(result) != 0 {
		t.Fatalf("patched output has %d bytes, want empty", len(result))
	}
}

func TestOptimalBlockSizeBounds(t *testing.T) {
	for _, size := range []uint64{
		0,
		1,
		1024,
		1024 * 1024,
		1024 * 1024 * 1024,
		10 * 1024 * 1024 * 1024,
		maxUint64,
	} {
		bs := OptimalBlockSizeForBaseLength(size)
		if bs < minBlockSize || bs > maxBlockSize {
			t.Fatalf("size=%d: block size=%d outside [%d, %d]", size, bs, minBlockSize, maxBlockSize)
		}
	}
}

func TestDeltifyRejectsInvalidSignature(t *testing.T) {
	engine := NewEngine(maxBlockSize)
	err := engine.Deltify(bytes.NewReader(nil), &Signature{}, func(Operation) error { return nil })
	if err == nil {
		t.Fatal("expected invalid signature error")
	}
}

func TestPatchRejectsOutOfRangeBlock(t *testing.T) {
	engine := NewEngine(maxBlockSize)
	err := engine.Patch(
		bytes.NewReader(deterministicBytes(2048)),
		[]Operation{{Type: OpBlock, BlockStart: 2, BlockCount: 1}},
		1024,
		&bytes.Buffer{},
	)
	if err == nil {
		t.Fatal("expected error for out-of-range block reference")
	}
}

func TestPatchRejectsZeroBlockCount(t *testing.T) {
	engine := NewEngine(maxBlockSize)
	err := engine.Patch(
		bytes.NewReader(deterministicBytes(2048)),
		[]Operation{{Type: OpBlock, BlockStart: 0}},
		1024,
		&bytes.Buffer{},
	)
	if err == nil {
		t.Fatal("expected error for zero-count block reference")
	}
}

func TestPatchRejectsUnknownOperation(t *testing.T) {
	engine := NewEngine(maxBlockSize)
	err := engine.Patch(
		bytes.NewReader(nil),
		[]Operation{{Type: OperationType(99)}},
		1024,
		&bytes.Buffer{},
	)
	if err == nil {
		t.Fatal("expected error for unknown operation")
	}
}

func BenchmarkDeltifySmallChange(b *testing.B) {
	sizes := []int{
		1024 * 1024,
		10 * 1024 * 1024,
	}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("%dMB", size/(1024*1024)), func(b *testing.B) {
			original := deterministicBytes(size)
			modified := cloneBytes(original)
			modified[size/2] ^= 0xff

			engine := NewEngine(maxBlockSize)
			sig, err := engine.GenerateSignature(bytes.NewReader(original), uint64(size))
			if err != nil {
				b.Fatal(err)
			}

			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := engine.Deltify(bytes.NewReader(modified), sig, func(Operation) error { return nil }); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func deltifyPatch(t *testing.T, original, modified []byte) ([]Operation, []byte, *Signature) {
	t.Helper()

	engine := NewEngine(maxBlockSize)
	sig, err := engine.GenerateSignature(bytes.NewReader(original), uint64(len(original)))
	if err != nil {
		t.Fatal(err)
	}

	var ops []Operation
	if err := engine.Deltify(bytes.NewReader(modified), sig, func(op Operation) error {
		ops = append(ops, op)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	var result bytes.Buffer
	if err := engine.Patch(bytes.NewReader(original), ops, sig.BlockSize, &result); err != nil {
		t.Fatal(err)
	}

	return ops, result.Bytes(), sig
}

func deterministicBytes(size int) []byte {
	r := rand.New(rand.NewSource(42))
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(r.Intn(256))
	}
	return data
}

func totalDataBytes(ops []Operation) int {
	total := 0
	for _, op := range ops {
		if op.Type == OpData {
			total += len(op.Data)
		}
	}
	return total
}

func totalBlockRefs(ops []Operation) uint64 {
	var total uint64
	for _, op := range ops {
		if op.Type == OpBlock {
			total += op.BlockCount
		}
	}
	return total
}
