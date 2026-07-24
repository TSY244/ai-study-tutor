/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { Lesson } from './lesson';
import type { PageInfo } from './pageInfo';

export interface LessonList {
  items: Lesson[];
  page_info: PageInfo;
}
