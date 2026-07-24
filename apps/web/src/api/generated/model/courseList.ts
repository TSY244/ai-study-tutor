/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { Course } from './course';
import type { PageInfo } from './pageInfo';

export interface CourseList {
  items: Course[];
  page_info: PageInfo;
}
