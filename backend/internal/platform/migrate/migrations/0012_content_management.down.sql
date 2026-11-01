ALTER TABLE documents DROP COLUMN deleted_at;
ALTER TABLE course_knowledge_points DROP COLUMN version;
ALTER TABLE materials DROP COLUMN version;
ALTER TABLE quizzes DROP COLUMN deleted_at, DROP COLUMN version;
