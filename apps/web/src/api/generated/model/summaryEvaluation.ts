/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { JobError } from './jobError';
import type { SummaryEvaluationStatus } from './summaryEvaluationStatus';

export interface SummaryEvaluation {
  id: string;
  lesson_id: string;
  status: SummaryEvaluationStatus;
  /**
     * @minimum 0
     * @maximum 1
     * @nullable
     */
  coverage_score: number | null;
  correct_points: string[];
  misconceptions: string[];
  missing_points: string[];
  suggestions: string[];
  error?: JobError | null;
  created_at: string;
  /** @nullable */
  completed_at?: string | null;
}
