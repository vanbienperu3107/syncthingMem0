// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package scanner

import (
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/syncthing/syncthing/lib/fs"
	"github.com/syncthing/syncthing/lib/protocol"
)

type Snapshot struct {
	mut   sync.RWMutex
	files map[string]protocol.FileInfo
}

func NewSnapshot() *Snapshot {
	return &Snapshot{
		files: make(map[string]protocol.FileInfo),
	}
}

func NewSnapshotFromFiles(files []protocol.FileInfo) *Snapshot {
	s := NewSnapshot()
	s.Update(files, nil)
	return s
}

func (s *Snapshot) Update(files []protocol.FileInfo, dirtyPaths map[string]struct{}) {
	if s == nil {
		return
	}

	s.mut.Lock()
	defer s.mut.Unlock()

	for path := range dirtyPaths {
		s.removeLocked(cleanSnapshotPath(path))
	}
	for _, file := range files {
		s.files[cleanSnapshotPath(file.Name)] = cloneFileInfo(file)
	}
}

func (s *Snapshot) Lookup(path string) (protocol.FileInfo, bool) {
	if s == nil {
		return protocol.FileInfo{}, false
	}

	s.mut.RLock()
	defer s.mut.RUnlock()

	file, ok := s.files[cleanSnapshotPath(path)]
	return cloneFileInfo(file), ok
}

func (s *Snapshot) SubtreeFiles(path string) []protocol.FileInfo {
	if s == nil {
		return nil
	}

	s.mut.RLock()
	defer s.mut.RUnlock()

	path = cleanSnapshotPath(path)
	var result []protocol.FileInfo
	for name, file := range s.files {
		if path == "" || name == path || fs.IsParent(name, path) {
			result = append(result, cloneFileInfo(file))
		}
	}
	slices.SortFunc(result, func(a, b protocol.FileInfo) int {
		return strings.Compare(a.Name, b.Name)
	})
	return result
}

func (s *Snapshot) Reset() {
	if s == nil {
		return
	}

	s.mut.Lock()
	defer s.mut.Unlock()
	s.files = make(map[string]protocol.FileInfo)
}

func (s *Snapshot) Size() int {
	if s == nil {
		return 0
	}

	s.mut.RLock()
	defer s.mut.RUnlock()
	return len(s.files)
}

func (s *Snapshot) removeLocked(path string) {
	delete(s.files, path)
	for name := range s.files {
		if fs.IsParent(name, path) {
			delete(s.files, name)
		}
	}
}

func cleanSnapshotPath(path string) string {
	path = filepath.ToSlash(path)
	path = strings.ReplaceAll(path, `\`, "/")
	path = strings.TrimPrefix(path, "./")
	if path == "." {
		return ""
	}
	return strings.Trim(path, "/")
}

func cloneFileInfo(file protocol.FileInfo) protocol.FileInfo {
	file.Blocks = slices.Clone(file.Blocks)
	for i := range file.Blocks {
		file.Blocks[i].Hash = slices.Clone(file.Blocks[i].Hash)
	}
	file.BlocksHash = slices.Clone(file.BlocksHash)
	file.PreviousBlocksHash = slices.Clone(file.PreviousBlocksHash)
	file.SymlinkTarget = slices.Clone(file.SymlinkTarget)
	return file
}
