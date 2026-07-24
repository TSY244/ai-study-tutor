-- 0001_init 回滚：按外键依赖逆序删除全部表。

DROP TABLE IF EXISTS legacy_import_records;
DROP TABLE IF EXISTS legacy_import_batches;
DROP TABLE IF EXISTS system_state;
DROP TABLE IF EXISTS backups;
DROP TABLE IF EXISTS idempotency_records;
DROP TABLE IF EXISTS summary_evaluations;
DROP TABLE IF EXISTS mastery_snapshots;
DROP TABLE IF EXISTS weaknesses;
DROP TABLE IF EXISTS attempt_answers;
DROP TABLE IF EXISTS attempts;
DROP TABLE IF EXISTS quiz_items;
DROP TABLE IF EXISTS quizzes;
DROP TABLE IF EXISTS questions;
DROP TABLE IF EXISTS documents;
DROP TABLE IF EXISTS ai_runs;
DROP TABLE IF EXISTS job_events;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS materials;
DROP TABLE IF EXISTS file_objects;
DROP TABLE IF EXISTS lessons;
DROP TABLE IF EXISTS courses;
