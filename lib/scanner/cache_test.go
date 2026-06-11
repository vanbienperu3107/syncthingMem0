// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package scanner

import (
	"testing"
	"time"

	"github.com/syncthing/syncthing/lib/protocol"
)

func TestDigestCacheLookup(t *testing.T) {
	cache := NewDigestCache(10, time.Hour)
	mtime := time.Now()
	file := protocol.FileInfo{
		Name:       "file.txt",
		Type:       protocol.FileInfoTypeFile,
		Size:       123,
		Blocks:     []protocol.BlockInfo{{Size: 123, Hash: []byte{1, 2, 3}}},
		BlocksHash: []byte{4, 5, 6},
	}

	cache.Set(`.\file.txt`, mtime, 123, file)
	got, ok := cache.Lookup("file.txt", mtime, 123)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.Name != file.Name || got.Size != file.Size || len(got.Blocks) != 1 {
		t.Fatalf("unexpected cached file: %#v", got)
	}

	got.Blocks[0].Hash[0] = 9
	again, ok := cache.Lookup("file.txt", mtime, 123)
	if !ok {
		t.Fatal("expected second cache hit")
	}
	if again.Blocks[0].Hash[0] == 9 {
		t.Fatal("cache lookup should return a cloned FileInfo")
	}
}

func TestDigestCacheMisses(t *testing.T) {
	cache := NewDigestCache(10, time.Hour)
	mtime := time.Now()
	cache.Set("file.txt", mtime, 123, protocol.FileInfo{Name: "file.txt", Size: 123})

	if _, ok := cache.Lookup("file.txt", mtime.Add(time.Nanosecond), 123); ok {
		t.Fatal("mtime mismatch should miss")
	}
	if _, ok := cache.Lookup("file.txt", mtime, 124); ok {
		t.Fatal("size mismatch should miss")
	}
	if _, ok := cache.Lookup("missing.txt", mtime, 123); ok {
		t.Fatal("missing path should miss")
	}
}

func TestDigestCacheTTLAndEviction(t *testing.T) {
	expiring := NewDigestCache(10, time.Nanosecond)
	mtime := time.Now()
	expiring.Set("file.txt", mtime, 1, protocol.FileInfo{Name: "file.txt", Size: 1})
	time.Sleep(time.Millisecond)
	if _, ok := expiring.Lookup("file.txt", mtime, 1); ok {
		t.Fatal("expired entry should miss")
	}

	cache := NewDigestCache(2, time.Hour)
	cache.Set("one.txt", mtime, 1, protocol.FileInfo{Name: "one.txt"})
	cache.Set("two.txt", mtime, 2, protocol.FileInfo{Name: "two.txt"})
	time.Sleep(time.Millisecond)
	cache.Set("three.txt", mtime, 3, protocol.FileInfo{Name: "three.txt"})
	if cache.Size() != 2 {
		t.Fatalf("cache size = %d, want 2", cache.Size())
	}
	if _, ok := cache.Lookup("one.txt", mtime, 1); ok {
		t.Fatal("oldest entry should have been evicted")
	}
}

func TestDigestCacheRemovePrefix(t *testing.T) {
	cache := NewDigestCache(10, time.Hour)
	mtime := time.Now()
	cache.Set("project/src/main.go", mtime, 1, protocol.FileInfo{Name: "project/src/main.go"})
	cache.Set("project/src/util.go", mtime, 1, protocol.FileInfo{Name: "project/src/util.go"})
	cache.Set("project/docs/readme.md", mtime, 1, protocol.FileInfo{Name: "project/docs/readme.md"})

	cache.RemovePrefix("project/src")

	if _, ok := cache.Lookup("project/src/main.go", mtime, 1); ok {
		t.Fatal("prefix entry should be removed")
	}
	if _, ok := cache.Lookup("project/docs/readme.md", mtime, 1); !ok {
		t.Fatal("outside prefix entry should remain")
	}
}
