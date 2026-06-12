// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package sqlite

import (
	"database/sql"
	"testing"

	"github.com/syncthing/syncthing/lib/protocol"
)

func TestLWWAncestorModifiedWinsOverConcurrentDelete(t *testing.T) {
	t.Parallel()

	sdb, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sdb.Close(); err != nil {
			t.Fatal(err)
		}
	})

	if err := sdb.SetFolderLWWReconciler(folderID, true); err != nil {
		t.Fatal(err)
	}

	baseVersion := protocol.Vector{}.Update(1)
	base := genFile("test1", 1, 1)
	base.ModifiedS = 100
	base.ModifiedNs = 0
	base.Version = baseVersion

	if err := sdb.Update(folderID, protocol.LocalDeviceID, []protocol.FileInfo{base}); err != nil {
		t.Fatal(err)
	}
	if err := sdb.Update(folderID, protocol.DeviceID{42}, []protocol.FileInfo{base}); err != nil {
		t.Fatal(err)
	}

	localModified := genFile("test1", 2, 2)
	localModified.ModifiedS = 200
	localModified.ModifiedNs = 0
	localModified.Version = baseVersion.Update(2)

	remoteDeleted := base
	remoteDeleted.ModifiedS = 300
	remoteDeleted.ModifiedNs = 0
	remoteDeleted.Version = baseVersion.Update(3)
	remoteDeleted.Sequence = 2
	remoteDeleted.Deleted = true
	remoteDeleted.Size = 0
	remoteDeleted.Blocks = nil
	remoteDeleted.BlocksHash = nil

	if err := sdb.Update(folderID, protocol.LocalDeviceID, []protocol.FileInfo{localModified}); err != nil {
		t.Fatal(err)
	}
	if err := sdb.Update(folderID, protocol.DeviceID{42}, []protocol.FileInfo{remoteDeleted}); err != nil {
		t.Fatal(err)
	}

	global, ok, err := sdb.GetGlobalFile(folderID, "test1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected global file")
	}
	if global.Deleted {
		t.Fatalf("modified side should win over concurrent delete with LWW enabled: %v", global)
	}
	if global.Size != localModified.Size {
		t.Fatalf("expected modified file size %d, got %d", localModified.Size, global.Size)
	}

	fdb, err := sdb.getFolderDB(folderID, false)
	if err != nil {
		t.Fatal(err)
	}
	var deleted bool
	if err := fdb.sql.Get(&deleted, `SELECT deleted FROM ancestor_entries WHERE name = ?`, "test1"); err != nil {
		t.Fatal(err)
	}
	if deleted {
		t.Fatal("ancestor must not be advanced to the losing delete")
	}
}

func TestLWWAncestorStoreCRUD(t *testing.T) {
	t.Parallel()

	sdb, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sdb.Close(); err != nil {
			t.Fatal(err)
		}
	})

	if err := sdb.SetFolderLWWReconciler(folderID, true); err != nil {
		t.Fatal(err)
	}

	fdb, err := sdb.getFolderDB(folderID, true)
	if err != nil {
		t.Fatal(err)
	}

	tx, err := fdb.sql.BeginTxx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	txp := &txPreparedStmts{Tx: tx}

	row := fileRow{
		Name:          "store.txt",
		Modified:      123,
		Size:          456,
		BlocklistHash: []byte("hash"),
	}
	if err := fdb.putAncestorLocked(txp, row); err != nil {
		t.Fatal(err)
	}
	got, err := fdb.getAncestorLocked(txp, "store.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected ancestor entry")
	}
	if got.ModifiedNanos != row.Modified || got.Size != row.Size || string(got.BlocklistHash) != string(row.BlocklistHash) {
		t.Fatalf("unexpected ancestor entry: %+v", got)
	}
	if err := fdb.deleteAncestorLocked(txp, "store.txt"); err != nil {
		t.Fatal(err)
	}
	got, err = fdb.getAncestorLocked(txp, "store.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected deleted ancestor, got %+v", got)
	}
}

func TestLWWAncestorMigrationSchema(t *testing.T) {
	t.Parallel()

	sdb, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sdb.Close(); err != nil {
			t.Fatal(err)
		}
	})

	fdb, err := sdb.getFolderDB(folderID, true)
	if err != nil {
		t.Fatal(err)
	}

	var tableName string
	err = fdb.sql.Get(&tableName, `SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'ancestor_entries'`)
	if err != nil {
		t.Fatal(err)
	}
	if tableName != "ancestor_entries" {
		t.Fatalf("expected ancestor_entries table, got %q", tableName)
	}

	var count int
	err = fdb.sql.Get(&count, `SELECT count(*) FROM pragma_table_info('ancestor_entries') WHERE name IN ('name', 'modified', 'size', 'blocklist_hash', 'deleted', 'updated_at')`)
	if err != nil {
		t.Fatal(err)
	}
	if count != 6 {
		t.Fatalf("ancestor_entries schema is missing columns, got %d", count)
	}

	var missing sql.NullString
	err = fdb.sql.Get(&missing, `SELECT name FROM pragma_table_info('ancestor_entries') WHERE pk > 0 ORDER BY pk LIMIT 1`)
	if err != nil {
		t.Fatal(err)
	}
	if missing.String != "name" {
		t.Fatalf("expected name primary key, got %q", missing.String)
	}
}

func TestLWWAncestorFlagOffKeepsExistingConflictOrdering(t *testing.T) {
	t.Parallel()

	sdb, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sdb.Close(); err != nil {
			t.Fatal(err)
		}
	})

	baseVersion := protocol.Vector{}.Update(1)
	base := genFile("test1", 1, 1)
	base.ModifiedS = 100
	base.ModifiedNs = 0
	base.Version = baseVersion

	if err := sdb.Update(folderID, protocol.LocalDeviceID, []protocol.FileInfo{base}); err != nil {
		t.Fatal(err)
	}
	if err := sdb.Update(folderID, protocol.DeviceID{42}, []protocol.FileInfo{base}); err != nil {
		t.Fatal(err)
	}

	localModified := genFile("test1", 2, 2)
	localModified.ModifiedS = 200
	localModified.ModifiedNs = 0
	localModified.Version = baseVersion.Update(2)

	remoteDeleted := base
	remoteDeleted.ModifiedS = 300
	remoteDeleted.ModifiedNs = 0
	remoteDeleted.Version = baseVersion.Update(3)
	remoteDeleted.Sequence = 2
	remoteDeleted.Deleted = true
	remoteDeleted.Size = 0
	remoteDeleted.Blocks = nil
	remoteDeleted.BlocksHash = nil

	if err := sdb.Update(folderID, protocol.LocalDeviceID, []protocol.FileInfo{localModified}); err != nil {
		t.Fatal(err)
	}
	if err := sdb.Update(folderID, protocol.DeviceID{42}, []protocol.FileInfo{remoteDeleted}); err != nil {
		t.Fatal(err)
	}

	global, ok, err := sdb.GetGlobalFile(folderID, "test1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected global file")
	}
	if !global.Deleted {
		t.Fatalf("flag off should keep existing mtime conflict ordering, got %v", global)
	}
}

func TestLWWAncestorOnlyOneSideChanged(t *testing.T) {
	t.Parallel()

	sdb, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sdb.Close(); err != nil {
			t.Fatal(err)
		}
	})

	if err := sdb.SetFolderLWWReconciler(folderID, true); err != nil {
		t.Fatal(err)
	}

	baseVersion := protocol.Vector{}.Update(1)
	base := genFile("test1", 1, 1)
	base.ModifiedS = 100
	base.ModifiedNs = 0
	base.Version = baseVersion

	if err := sdb.Update(folderID, protocol.LocalDeviceID, []protocol.FileInfo{base}); err != nil {
		t.Fatal(err)
	}
	if err := sdb.Update(folderID, protocol.DeviceID{42}, []protocol.FileInfo{base}); err != nil {
		t.Fatal(err)
	}

	remoteModified := genFile("test1", 2, 2)
	remoteModified.ModifiedS = 200
	remoteModified.ModifiedNs = 0
	remoteModified.Version = baseVersion.Update(3)
	remoteModified.Sequence = 2
	if err := sdb.Update(folderID, protocol.DeviceID{42}, []protocol.FileInfo{remoteModified}); err != nil {
		t.Fatal(err)
	}

	global, ok, err := sdb.GetGlobalFile(folderID, "test1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected global file")
	}
	if global.Size != remoteModified.Size {
		t.Fatalf("one-sided change should win over ancestor side: got size %d", global.Size)
	}
}
