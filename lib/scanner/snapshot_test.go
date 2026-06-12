// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package scanner

import (
	"slices"
	"testing"

	"github.com/syncthing/syncthing/lib/protocol"
)

func TestSnapshotLookupAndSubtree(t *testing.T) {
	snapshot := NewSnapshotFromFiles([]protocol.FileInfo{
		{Name: "dir", Type: protocol.FileInfoTypeDirectory},
		{Name: "dir/a.txt", Type: protocol.FileInfoTypeFile, Size: 1},
		{Name: "dir/nested/b.txt", Type: protocol.FileInfoTypeFile, Size: 2},
		{Name: "other.txt", Type: protocol.FileInfoTypeFile, Size: 3},
	})

	if snapshot.Size() != 4 {
		t.Fatalf("snapshot size = %d, want 4", snapshot.Size())
	}

	file, ok := snapshot.Lookup(`.\dir\a.txt`)
	if !ok {
		t.Fatal("expected lookup hit")
	}
	if file.Name != "dir/a.txt" {
		t.Fatalf("lookup name = %q, want dir/a.txt", file.Name)
	}

	subtree := snapshot.SubtreeFiles("dir")
	names := make([]string, 0, len(subtree))
	for _, file := range subtree {
		names = append(names, file.Name)
	}
	want := []string{"dir", "dir/a.txt", "dir/nested/b.txt"}
	if !slices.Equal(names, want) {
		t.Fatalf("subtree names = %v, want %v", names, want)
	}
}

func TestSnapshotUpdateRemovesDirtySubtree(t *testing.T) {
	snapshot := NewSnapshotFromFiles([]protocol.FileInfo{
		{Name: "dir/a.txt", Type: protocol.FileInfoTypeFile, Size: 1},
		{Name: "dir/nested/b.txt", Type: protocol.FileInfoTypeFile, Size: 2},
		{Name: "keep.txt", Type: protocol.FileInfoTypeFile, Size: 3},
	})

	snapshot.Update([]protocol.FileInfo{
		{Name: "dir/new.txt", Type: protocol.FileInfoTypeFile, Size: 4},
	}, map[string]struct{}{"dir": {}})

	if _, ok := snapshot.Lookup("dir/a.txt"); ok {
		t.Fatal("dirty subtree entry should have been removed")
	}
	if _, ok := snapshot.Lookup("dir/nested/b.txt"); ok {
		t.Fatal("dirty nested subtree entry should have been removed")
	}
	if _, ok := snapshot.Lookup("keep.txt"); !ok {
		t.Fatal("unrelated entry should remain")
	}
	if _, ok := snapshot.Lookup("dir/new.txt"); !ok {
		t.Fatal("new entry should be recorded")
	}
}

func TestIsDirtyOrAncestor(t *testing.T) {
	dirty := map[string]struct{}{
		"src/main.go":    {},
		"docs/api/v2.md": {},
	}

	tests := []struct {
		path string
		want bool
	}{
		{"src/main.go", true},
		{"src", true},
		{"docs/api", true},
		{"lib/util.go", false},
		{"src_backup/main.go", false},
		{"docs2/api/v2.md", false},
	}

	for _, tc := range tests {
		if got := IsDirtyOrAncestor(tc.path, dirty); got != tc.want {
			t.Fatalf("IsDirtyOrAncestor(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
