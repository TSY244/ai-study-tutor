/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { PageInfo } from './pageInfo';
import type { QuizSummary } from './quizSummary';

export interface QuizList {
  items: QuizSummary[];
  page_info: PageInfo;
}
