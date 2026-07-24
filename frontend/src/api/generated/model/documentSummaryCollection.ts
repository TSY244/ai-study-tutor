/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { DocumentSummary } from './documentSummary';

export interface DocumentSummaryCollection {
  /** 当前发布版本；每个 DocumentKind 最多一项，不使用游标分页。 */
  items: DocumentSummary[];
}
