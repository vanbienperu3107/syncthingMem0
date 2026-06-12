// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package sqlite

import (
	"bytes"
	"database/sql"
	"errors"
	"time"

	"github.com/syncthing/syncthing/internal/db"
)

func (s *folderDB) getAncestorLocked(txp *txPreparedStmts, name string) (*db.AncestorEntry, error) {
	var entry db.AncestorEntry
	err := txp.Get(&entry, `
		SELECT name, modified AS modifiednanos, size, blocklist_hash AS blocklisthash, deleted, updated_at AS updatednanos
		FROM ancestor_entries
		WHERE name = ?
	`, name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, wrap(err, "get ancestor")
	}
	return &entry, nil
}

func (s *folderDB) putAncestorLocked(txp *txPreparedStmts, row fileRow) error {
	_, err := txp.Exec(`
		INSERT INTO ancestor_entries (name, modified, size, blocklist_hash, deleted, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			modified = excluded.modified,
			size = excluded.size,
			blocklist_hash = excluded.blocklist_hash,
			deleted = excluded.deleted,
			updated_at = excluded.updated_at
	`, row.Name, row.Modified, row.Size, row.BlocklistHash, row.Deleted, time.Now().UnixNano())
	return wrap(err, "put ancestor")
}

func (s *folderDB) deleteAncestorLocked(txp *txPreparedStmts, name string) error {
	_, err := txp.Exec(`
		DELETE FROM ancestor_entries
		WHERE name = ?
	`, name)
	return wrap(err, "delete ancestor")
}

func rowMatchesAncestor(row fileRow, ancestor *db.AncestorEntry) bool {
	if ancestor == nil {
		return false
	}
	if row.Deleted && ancestor.Deleted {
		return true
	}
	if row.Deleted != ancestor.Deleted {
		return false
	}
	return row.Modified == ancestor.ModifiedNanos &&
		row.Size == ancestor.Size &&
		bytes.Equal(row.BlocklistHash, ancestor.BlocklistHash)
}

func rowsEqualByAncestorMetadata(a, b fileRow) bool {
	if a.Deleted && b.Deleted {
		return true
	}
	if a.Deleted != b.Deleted {
		return false
	}
	return a.Modified == b.Modified &&
		a.Size == b.Size &&
		bytes.Equal(a.BlocklistHash, b.BlocklistHash)
}
