-- Copyright (C) 2026 The Syncthing Authors.
--
-- This Source Code Form is subject to the terms of the Mozilla Public
-- License, v. 2.0. If a copy of the MPL was not distributed with this file,
-- You can obtain one at https://mozilla.org/MPL/2.0/.

CREATE TABLE IF NOT EXISTS ancestor_entries (
    name TEXT NOT NULL PRIMARY KEY COLLATE BINARY,
    modified INTEGER NOT NULL,
    size INTEGER NOT NULL,
    blocklist_hash BLOB,
    deleted INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL
)
;
