/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { KnowledgePointImportance } from './knowledgePointImportance';

export interface KnowledgePoint {
  /** @minimum 1 */
  version?: number;
  id: string;
  course_id: string;
  lesson_id: string;
  lesson_title: string;
  label: string;
  explanation: string;
  learning_objective: string;
  source_excerpt: string;
  importance: KnowledgePointImportance;
  updated_at: string;
}
