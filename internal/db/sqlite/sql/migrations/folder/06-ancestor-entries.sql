CREATE TABLE IF NOT EXISTS ancestor_entries (
    name TEXT NOT NULL PRIMARY KEY COLLATE BINARY,
    modified INTEGER NOT NULL,
    size INTEGER NOT NULL,
    blocklist_hash BLOB,
    deleted INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL
)
;
