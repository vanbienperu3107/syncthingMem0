// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package rsync

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"testing"
)

func deterministicBytesSeed(size int, seed int64) []byte {
	r := rand.New(rand.NewSource(seed))
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(r.Intn(256))
	}
	return data
}

func deltaRoundTrip(t *testing.T, base, target []byte) []byte {
	t.Helper()
	sig, err := SignatureBytes(base)
	if err != nil {
		t.Fatal(err)
	}
	delta, err := DeltaBytes(target, sig)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ApplyDelta(base, delta)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, target) {
		t.Fatalf("round-trip mismatch: got %d bytes, want %d", len(got), len(target))
	}
	return delta
}

func TestWireRoundTripCases(t *testing.T) {
	base := deterministicBytes(256 * 1024)

	cases := []struct {
		name   string
		base   []byte
		target []byte
	}{
		{"identical", base, cloneBytes(base)},
		{"empty-base", nil, deterministicBytes(64 * 1024)},
		{"empty-target", base, nil},
		{"both-empty", nil, nil},
		{"small-edit", base, func() []byte {
			m := cloneBytes(base)
			copy(m[100*1024:101*1024], bytes.Repeat([]byte{0x5a}, 1024))
			return m
		}()},
		{"prefix-insert", base, append([]byte("prefix-bytes-here"), base...)},
		{"disjoint", base, deterministicBytesSeed(200*1024, 99)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deltaRoundTrip(t, tc.base, tc.target)
		})
	}
}

func TestWireDeltaSmallerForLocalizedEdit(t *testing.T) {
	// The whole point of the delta path: a localized edit inside a large block
	// should produce a delta far smaller than the block itself.
	base := deterministicBytes(4 * 1024 * 1024)
	target := cloneBytes(base)
	copy(target[2*1024*1024:2*1024*1024+512], bytes.Repeat([]byte{0xa5}, 512))

	delta := deltaRoundTrip(t, base, target)

	if len(delta) > len(target)/8 {
		t.Fatalf("delta = %d bytes, want much smaller than target %d", len(delta), len(target))
	}
}

func TestWireSignatureCodecRoundTrip(t *testing.T) {
	base := deterministicBytes(300 * 1024)
	e := NewEngine(0)
	sig, err := e.GenerateSignature(bytes.NewReader(base), uint64(len(base)))
	if err != nil {
		t.Fatal(err)
	}

	got, err := decodeSignature(encodeSignature(sig))
	if err != nil {
		t.Fatal(err)
	}
	if got.BlockSize != sig.BlockSize || got.LastBlockSize != sig.LastBlockSize {
		t.Fatalf("block sizes differ: got %+v want %+v", got, sig)
	}
	if len(got.Hashes) != len(sig.Hashes) {
		t.Fatalf("hash count %d, want %d", len(got.Hashes), len(sig.Hashes))
	}
	for i := range sig.Hashes {
		if got.Hashes[i] != sig.Hashes[i] {
			t.Fatalf("hash %d differs", i)
		}
	}
}

func TestWireDeltaCodecRoundTrip(t *testing.T) {
	ops := []Operation{
		{Type: OpBlock, BlockStart: 0, BlockCount: 3},
		{Type: OpData, Data: []byte("hello world")},
		{Type: OpBlock, BlockStart: 7, BlockCount: 1},
		{Type: OpData, Data: nil},
	}
	blockSize, got, err := decodeDelta(encodeDelta(4096, ops))
	if err != nil {
		t.Fatal(err)
	}
	if blockSize != 4096 {
		t.Fatalf("block size %d, want 4096", blockSize)
	}
	if len(got) != len(ops) {
		t.Fatalf("op count %d, want %d", len(got), len(ops))
	}
	for i := range ops {
		if got[i].Type != ops[i].Type || got[i].BlockStart != ops[i].BlockStart ||
			got[i].BlockCount != ops[i].BlockCount || !bytes.Equal(got[i].Data, ops[i].Data) {
			t.Fatalf("op %d differs: got %+v want %+v", i, got[i], ops[i])
		}
	}
}

func TestWireDecodeSignatureRejectsCorrupt(t *testing.T) {
	valid := encodeSignature(&Signature{BlockSize: 1024, LastBlockSize: 512, Hashes: []BlockHash{{WeakHash: 1}}})

	cases := map[string][]byte{
		"empty":       nil,
		"bad-version": {99},
		"truncated":   valid[:len(valid)-3],
		"huge-count": func() []byte {
			b := []byte{sigCodecVersion}
			b = binary.AppendUvarint(b, 1024)
			b = binary.AppendUvarint(b, 0)
			return binary.AppendUvarint(b, 1<<40)
		}(),
	}
	for name, b := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeSignature(b); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestWireDecodeDeltaRejectsCorrupt(t *testing.T) {
	cases := map[string][]byte{
		"empty":       nil,
		"bad-version": {99},
		"unknown-op": func() []byte {
			b := []byte{deltaCodecVersion}
			b = binary.AppendUvarint(b, 4096)
			b = binary.AppendUvarint(b, 1)
			return append(b, 0xff) // op type 255
		}(),
		"huge-op-count": func() []byte {
			b := []byte{deltaCodecVersion}
			b = binary.AppendUvarint(b, 4096)
			return binary.AppendUvarint(b, 1<<40)
		}(),
		"oversized-data": func() []byte {
			b := []byte{deltaCodecVersion}
			b = binary.AppendUvarint(b, 4096)
			b = binary.AppendUvarint(b, 1)
			b = append(b, byte(OpData))
			return binary.AppendUvarint(b, maxDataOperationSize+1)
		}(),
	}
	for name, b := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := decodeDelta(b); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestWirePublicAPIRejectsCorrupt(t *testing.T) {
	if _, err := DeltaBytes([]byte("x"), []byte{99}); err == nil {
		t.Fatal("DeltaBytes accepted corrupt signature")
	}
	if _, err := ApplyDelta([]byte("x"), []byte{99}); err == nil {
		t.Fatal("ApplyDelta accepted corrupt delta")
	}
}
