/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { WeaknessEvidenceItem } from './weaknessEvidenceItem';

export interface Weakness {
  id: string;
  course_id: string;
  /** @nullable */
  lesson_id?: string | null;
  concept_key: string;
  concept_label: string;
  description: string;
  evidence: WeaknessEvidenceItem[];
  /**
     * @minimum 1
     * @maximum 5
     */
  severity: number;
  /** @minimum 0 */
  review_count: number;
  mastered: boolean;
  version: number;
  first_seen_at: string;
  last_seen_at: string;
}
