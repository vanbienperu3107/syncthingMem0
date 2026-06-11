// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package scanner

import (
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/syncthing/syncthing/lib/fs"
	"github.com/syncthing/syncthing/lib/protocol"
)

type DigestCache struct {
	mut        sync.RWMutex
	entries    map[string]DigestEntry
	maxEntries int
	ttl        time.Duration
}

type DigestEntry struct {
	File     protocol.FileInfo
	ModTime  time.Time
	Size     int64
	CachedAt time.Time
}

type CacheStats struct {
	Entries    int
	MaxEntries int
	TTL        time.Duration
}

func NewDigestCache(maxEntries int, ttl time.Duration) *DigestCache {
	return &DigestCache{
		entries:    make(map[string]DigestEntry),
		maxEntries: maxEntries,
		ttl:        ttl,
	}
}

func (c *DigestCache) Lookup(path string, modTime time.Time, size int64) (protocol.FileInfo, bool) {
	if c == nil {
		return protocol.FileInfo{}, false
	}

	c.mut.RLock()
	defer c.mut.RUnlock()

	entry, ok := c.entries[cleanCachePath(path)]
	if !ok {
		return protocol.FileInfo{}, false
	}
	if c.ttl > 0 && time.Since(entry.CachedAt) > c.ttl {
		return protocol.FileInfo{}, false
	}
	if entry.Size != size || !entry.ModTime.Equal(modTime) {
		return protocol.FileInfo{}, false
	}
	return cloneFileInfo(entry.File), true
}

func (c *DigestCache) Set(path string, modTime time.Time, size int64, file protocol.FileInfo) {
	if c == nil {
		return
	}

	c.mut.Lock()
	defer c.mut.Unlock()

	_, exists := c.entries[cleanCachePath(path)]
	if c.maxEntries > 0 && !exists && len(c.entries) >= c.maxEntries {
		c.evictOldestLocked()
	}
	c.entries[cleanCachePath(path)] = DigestEntry{
		File:     cloneFileInfo(file),
		ModTime:  modTime,
		Size:     size,
		CachedAt: time.Now(),
	}
}

func (c *DigestCache) Remove(path string) {
	if c == nil {
		return
	}

	c.mut.Lock()
	defer c.mut.Unlock()
	delete(c.entries, cleanCachePath(path))
}

func (c *DigestCache) RemovePrefix(prefix string) {
	if c == nil {
		return
	}

	c.mut.Lock()
	defer c.mut.Unlock()

	prefix = cleanCachePath(prefix)
	for path := range c.entries {
		if path == prefix || fs.IsParent(path, prefix) {
			delete(c.entries, path)
		}
	}
}

func (c *DigestCache) Clear() {
	if c == nil {
		return
	}

	c.mut.Lock()
	defer c.mut.Unlock()
	c.entries = make(map[string]DigestEntry)
}

func (c *DigestCache) Size() int {
	if c == nil {
		return 0
	}

	c.mut.RLock()
	defer c.mut.RUnlock()
	return len(c.entries)
}

func (c *DigestCache) Stats() CacheStats {
	if c == nil {
		return CacheStats{}
	}

	c.mut.RLock()
	defer c.mut.RUnlock()
	return CacheStats{
		Entries:    len(c.entries),
		MaxEntries: c.maxEntries,
		TTL:        c.ttl,
	}
}

func (c *DigestCache) evictOldestLocked() {
	var oldestPath string
	var oldestTime time.Time
	first := true
	for path, entry := range c.entries {
		if first || entry.CachedAt.Before(oldestTime) {
			oldestPath = path
			oldestTime = entry.CachedAt
			first = false
		}
	}
	delete(c.entries, oldestPath)
}

func cleanCachePath(path string) string {
	path = filepath.ToSlash(path)
	path = strings.ReplaceAll(path, `\`, "/")
	path = strings.TrimPrefix(path, "./")
	return strings.Trim(path, "/")
}

func IsDirtyOrAncestor(path string, dirtyPaths map[string]struct{}) bool {
	path = cleanCachePath(path)
	if _, ok := dirtyPaths[path]; ok {
		return true
	}
	for dirtyPath := range dirtyPaths {
		dirtyPath = cleanCachePath(dirtyPath)
		if path == dirtyPath || fs.IsParent(dirtyPath, path) || fs.IsParent(path, dirtyPath) {
			return true
		}
	}
	return false
}
