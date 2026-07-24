/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { Material } from './material';
import type { PageInfo } from './pageInfo';

export interface MaterialList {
  items: Material[];
  page_info: PageInfo;
}
