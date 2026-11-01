CREATE TABLE course_knowledge_points (
    id uuid PRIMARY KEY,
    course_id uuid NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    lesson_id uuid NOT NULL REFERENCES lessons(id) ON DELETE CASCADE,
    concept_key text NOT NULL,
    label varchar(120) NOT NULL,
    explanation text NOT NULL,
    learning_objective text NOT NULL,
    source_excerpt text NOT NULL,
    importance text NOT NULL CHECK (importance IN ('core','supporting')),
    active boolean NOT NULL DEFAULT true,
    source_job_id uuid REFERENCES jobs(id) ON DELETE SET NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (lesson_id,concept_key)
);
CREATE INDEX course_knowledge_points_course ON course_knowledge_points(course_id,lesson_id) WHERE active;
