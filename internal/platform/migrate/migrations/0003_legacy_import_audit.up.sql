-- B9: make the legacy-import batch fingerprint explicit and prevent two
-- independently started importer processes from publishing the same manifest.
ALTER TABLE legacy_import_batches
    ADD COLUMN IF NOT EXISTS source_root_fingerprint TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS resolution_sha256 CHAR(64) NOT NULL DEFAULT ''
        CHECK (resolution_sha256 = '' OR resolution_sha256 ~ '^[0-9a-f]{64}$');

DROP INDEX IF EXISTS uq_legacy_import_batches_manifest;

CREATE UNIQUE INDEX IF NOT EXISTS uq_legacy_import_batches_manifest_resolution
    ON legacy_import_batches (source_manifest_hash, resolution_sha256);
