ALTER TABLE quizzes ADD COLUMN version bigint NOT NULL DEFAULT 1 CHECK(version>0), ADD COLUMN deleted_at timestamptz;
ALTER TABLE materials ADD COLUMN version bigint NOT NULL DEFAULT 1 CHECK(version>0);
ALTER TABLE course_knowledge_points ADD COLUMN version bigint NOT NULL DEFAULT 1 CHECK(version>0);
ALTER TABLE documents ADD COLUMN deleted_at timestamptz;
