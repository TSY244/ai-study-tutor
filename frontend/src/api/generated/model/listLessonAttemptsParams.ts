/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { CursorParameter } from './cursorParameter';
import type { LimitParameter } from './limitParameter';

export type ListLessonAttemptsParams = {
cursor?: CursorParameter;
/**
 * @minimum 1
 * @maximum 100
 */
limit?: LimitParameter;
};
