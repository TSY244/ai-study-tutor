/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { KnowledgePointCreateImportance } from './knowledgePointCreateImportance';

export interface KnowledgePointCreate {
  /** @minLength 1 */
  label: string;
  /**
     * @minLength 1
     * @maxLength 2000
     */
  explanation: string;
  /**
     * @minLength 1
     * @maxLength 1000
     */
  learning_objective: string;
  /**
     * @minLength 1
     * @maxLength 1000
     */
  source_excerpt: string;
  /** @minLength 1 */
  importance: KnowledgePointCreateImportance;
}
