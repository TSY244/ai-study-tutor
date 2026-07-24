/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { AttemptAnswer } from './attemptAnswer';
import type { AttemptStatus } from './attemptStatus';
import type { Question } from './question';

export interface Attempt {
  id: string;
  quiz_id: string;
  status: AttemptStatus;
  version: number;
  /** @nullable */
  auto_score?: number | null;
  /** @nullable */
  final_score?: number | null;
  /** @nullable */
  max_score?: number | null;
  /** @nullable */
  grading_job_id?: string | null;
  /** @nullable */
  grading_error_code?: string | null;
  /** @nullable */
  grading_error_detail?: string | null;
  /** 来自不可变 Quiz 的有序学生可见题目快照。 */
  questions: Question[];
  answers: AttemptAnswer[];
  started_at: string;
  /** @nullable */
  submitted_at?: string | null;
  /** @nullable */
  graded_at?: string | null;
}
