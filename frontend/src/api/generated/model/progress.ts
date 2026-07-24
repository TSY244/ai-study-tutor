/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { ProgressPoint } from './progressPoint';
import type { ProgressScopeType } from './progressScopeType';

export interface Progress {
  scope_type: ProgressScopeType;
  scope_id: string;
  /**
     * @minimum 0
     * @maximum 1
     * @nullable
     */
  mastery: number | null;
  /** @nullable */
  formula_version: string | null;
  /** @minimum 0 */
  completed_attempts: number;
  /** @minimum 0 */
  active_weaknesses: number;
  trend: ProgressPoint[];
}
