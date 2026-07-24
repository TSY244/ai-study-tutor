DROP INDEX IF EXISTS uq_legacy_import_batches_manifest_resolution;
ALTER TABLE legacy_import_batches
    DROP COLUMN IF EXISTS source_root_fingerprint,
    DROP COLUMN IF EXISTS resolution_sha256;
