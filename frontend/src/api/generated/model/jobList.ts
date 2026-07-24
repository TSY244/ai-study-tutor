/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { Job } from './job';
import type { PageInfo } from './pageInfo';

export interface JobList {
  items: Job[];
  page_info: PageInfo;
}
