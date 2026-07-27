/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { AttemptSummary } from './attemptSummary';
import type { PageInfo } from './pageInfo';

export interface AttemptList {
  items: AttemptSummary[];
  page_info: PageInfo;
}
