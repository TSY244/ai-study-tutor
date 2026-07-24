/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { DocumentSummary } from './documentSummary';
import type { PageInfo } from './pageInfo';

export interface DocumentList {
  items: DocumentSummary[];
  page_info: PageInfo;
}
